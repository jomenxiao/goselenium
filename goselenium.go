package main

import (
	"github.com/ngaut/log"
	"github.com/tebeka/selenium"
	"sync"
)

const (
	// These paths will be different on your system.
	seleniumPath = "vendor/selenium-server-standalone-3.4.jar"
	// geckoDriverPath = "vendor/geckodriver-v0.18.0-linux64"
	portStart = 8080

	loginButton            = `//button[@type="submit"]`
	shareIcon              = `//a[@bs-tooltip="'Share dashboard'"]`
	snapshotFont           = `//i[@class="icon-gf icon-gf-snapshot"]`
	externalEnabledButtion = `//button[@ng-if="externalEnabled"]`
	shareElement           = `//a[@class="large share-modal-link"]`
	existPanelElementFlag  = `//canvas[@class="flot-overlay"]`
)

var (
	pngDir       = "SnapshotDir"
	BaseHTTP     = "http://192.168.2.188:3000"
	LoginURL     = "/login"
	UserName     = "admin"
	PassWord     = "admin"
	dashboardAPI = "/api/search?query="
	panelAPI     = "/api/annotations?limit=10000"
	FromTime     = 1510535835702
	ToTime       = 1511075844572
)

type Run struct {
	svrs          chan *selenium.Service
	wds           chan selenium.WebDriver
	dashboards    []string
	dashboardUrls chan string
	panelUrls     chan string
	screenDir     string
}

func (r *Run) schedule() {
	for wd := range r.wds {
		go func(wd selenium.WebDriver) {
			for {
				if len(r.dashboardUrls) > 0 {
					r.getShareURL(wd, <-r.dashboardUrls, false)
				}
				if len(r.panelUrls) > 0 {
					r.getShareURL(wd, <-r.panelUrls, true)
				}
				if len(r.panelUrls) == 0 && len(r.dashboardUrls) == 0 {
					break
				}
			}
		}(wd)
	}

}

func main() {
	var wgS sync.WaitGroup
	runNum := getCPUNum() / 2
	r := &Run{
		svrs:          make(chan *selenium.Service, runNum),
		wds:           make(chan selenium.WebDriver, runNum),
		dashboardUrls: make(chan string, 10),
		panelUrls:     make(chan string, 10000),
	}
	if err := r.prefixWork(); err != nil || r.screenDir == "" {
		log.Errorf("can not create screenshot directory with err %v", err)
		return
	}
	for i := 0; i < runNum; i++ {
		wgS.Add(1)
		go func() {
			r.createChromes(portStart + i)
			wgS.Done()
		}()
	}
	wgS.Wait()
	defer func() {
		for wd := range r.wds {
			wd.Quit()
		}
		for svr := range r.svrs {
			svr.Stop()
		}
	}()

	wd := <-r.wds
	if err := r.getDbashboards(wd); err != nil {
		panic(err)
	}
	if err := r.getPanels(wd); err != nil {
		panic(err)
	}
	r.wds <- wd

	r.schedule()
}
