package main

import (
	"flag"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/ngaut/log"
	"github.com/tebeka/selenium"
	"golang.org/x/net/context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	seleniumFile = "selenium-server-standalone-3.4.jar"
	pngDir       = "SnapshotDir"
	// geckoDriverPath = "vendor/geckodriver-v0.18.0-linux64"
	portStart = 8080

	loginButton            = `//button[@type="submit"]`
	shareIcon              = `//a[@bs-tooltip="'Share dashboard'"]`
	snapshotFont           = `//i[@class="icon-gf icon-gf-snapshot"]`
	externalEnabledButtion = `//button[@ng-if="externalEnabled"]`
	shareElement           = `//a[@class="large share-modal-link"]`
	existPanelElementFlag  = `//canvas[@class="flot-overlay"]`

	loginPath    = "/login"
	dashboardAPI = "/api/search?query="
	panelAPI     = "/api/annotations?limit=10000"

	timeFormat = "2006-01-02 15:04:05"
)

var (
	BaseHTTP string
	UserName string
	PassWord string
	From     string
	To       string
	FromTime int64
	ToTime   int64
)

type Run struct {
	ctx           context.Context
	cancel        context.CancelFunc
	svrs          chan *selenium.Service
	wds           chan selenium.WebDriver
	dashboards    []string
	dashboardUrls chan string
	panelUrls     chan string
	screenDir     string
	seleniumPath  string
}

func init() {
	var err error

	flag.StringVar(&BaseHTTP, "grafana_address", "http://192.168.2.188:3000", "input grafana_address")
	flag.StringVar(&UserName, "grafana_username", "admin", "granfan username")
	flag.StringVar(&PassWord, "grafana_password", "admin", "grafana password")
	flag.StringVar(&From, "grafana_starttime", time.Now().Local().AddDate(0, 0, -3).Format(timeFormat), "input start time, default is 3 days ago")
	flag.StringVar(&To, "grafana_endtime", time.Now().Local().Format(timeFormat), "input end time,default is now")

	ft, err := dateparse.ParseLocal(From)
	if err != nil {
		panic(fmt.Sprintf("start time is error %v", err))
	}
	FromTime = ft.Unix() * 1000
	et, err := dateparse.ParseLocal(To)
	if err != nil {
		panic(fmt.Sprintf("end time is error %v", err))
	}
	ToTime = et.Unix() * 1000
}

func main() {
	flag.Parse()

	var wgS, wgForever sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	runNum := getCPUNum() / 2
	r := &Run{
		ctx:           ctx,
		cancel:        cancel,
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

	r.getAPIsInfo()
	go r.schedule()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	wgForever.Add(1)
	go func() {
		sig := <-sc
		log.Errorf("Got signal [%d] to exit.", sig)
		r.cancel()
		wgForever.Done()
		defer func() {
			for wd := range r.wds {
				wd.Quit()
			}
			for svr := range r.svrs {
				svr.Stop()
			}
		}()

	}()

	wgForever.Wait()
}
