package main

import (
	"encoding/json"
	"fmt"
	"github.com/juju/errors"
	"github.com/tebeka/selenium"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func (r *Run) getAPIsInfo() {
	wd := <-r.wds
	if err := r.getDbashboards(wd); err != nil {
		panic(err)
	}
	if err := r.getPanels(wd); err != nil {
		panic(err)
	}
	r.wds <- wd

}

func (r *Run) savePng(wd selenium.WebDriver, name string) error {
	time.Sleep(2 * time.Second)
	screen, err := wd.Screenshot()
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("%s.png", name)
	return ioutil.WriteFile(filepath.Join(r.screenDir, fileName), screen, 0644)
}

func (r *Run) loginGrafana(wd selenium.WebDriver, port int) error {
	loginURL := fmt.Sprintf("%s%s", BaseHTTP, loginPath)
	if err := wd.Get(loginURL); err != nil {
		return err
	}
	elemU, errU := wd.FindElement(selenium.ByName, "username")
	if errU != nil {
		return errU
	}
	errK := elemU.SendKeys(UserName)
	if errK != nil {
		return errK
	}
	elemP, errP := wd.FindElement(selenium.ByName, "password")
	if errP != nil {
		return errP
	}
	errPK := elemP.SendKeys(PassWord)
	if errPK != nil {
		return errPK
	}
	elemButton, errB := wd.FindElement(selenium.ByXPATH, loginButton)
	if errB != nil {
		return nil
	}
	if err := elemButton.Click(); err != nil {
		return err
	}
	return r.savePng(wd, fmt.Sprintf("grafana_login_%d", port))
}

func (r *Run) createChromes(port int) {
	opts := []selenium.ServiceOption{
	//selenium.Output(os.Stderr), // Output debug information to STDERR.
	}
	//selenium.SetDebug(true)2
	service, err := selenium.NewSeleniumService(r.seleniumPath, port, opts...)
	if err != nil {
		panic(err) // panic is used only as an example and is not otherwise recommended.
	}
	r.svrs <- service
	// Connect to the WebDriver instance running locally.
	caps := selenium.Capabilities{"browserName": "chrome"}
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		panic(err)
	}
	if err := r.loginGrafana(wd, port); err != nil {
		panic(err)
	}
	r.wds <- wd
}

func (r *Run) getDbashboards(wd selenium.WebDriver) error {
	dashboardURL := fmt.Sprintf("%s%s", BaseHTTP, dashboardAPI)
	if err := wd.Get(dashboardURL); err != nil {
		return err
	}

	re, _ := regexp.Compile("\\[\\{.*\\}\\]")
	ps, errPS := wd.PageSource()
	if errPS != nil {
		return errPS
	}
	useStr := re.FindString(ps)
	if len(useStr) == 0 {
		return errors.New("can not get dashbord information")
	}
	type Dash struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		URI   string `json:"uri"`
	}
	dashboards := make([]Dash, 0)
	if err := json.Unmarshal([]byte(useStr), &dashboards); err != nil {
		return err
	}

	for _, dashboard := range dashboards {
		if strings.Contains(dashboard.Title, "PD") || strings.Contains(dashboard.Title, "TiDB") || strings.Contains(dashboard.Title, "TiKV") {
			r.dashboards = append(r.dashboards, fmt.Sprintf("%s /dashboard/%s", strings.Replace(dashboard.Title, " ", "_", -1), dashboard.URI))
		}
		r.dashboardUrls <- fmt.Sprintf("%s %s/dashboard/%s?from=%d&to=%d", strings.Replace(dashboard.Title, " ", "_", -1), BaseHTTP, dashboard.URI, FromTime, ToTime)
	}
	return nil
}

func (r *Run) getPanels(wd selenium.WebDriver) error {
	panelURL := fmt.Sprintf("%s%s", BaseHTTP, panelAPI)
	if err := wd.Get(panelURL); err != nil {
		return err
	}

	re, _ := regexp.Compile("\\[\\{.*\\}\\]")
	ps, errPS := wd.PageSource()
	if errPS != nil {
		return errPS
	}
	useStr := re.FindString(ps)
	if len(useStr) == 0 {
		return errors.New("can not get panel information")
	}

	type panel struct {
		DashboardID int    `json:"dashboardId"`
		PanelID     int    `json:"panelId"`
		Title       string `json:"title"`
	}
	panels := make([]panel, 0)
	if err := json.Unmarshal([]byte(useStr), &panels); err != nil {
		return err
	}
	for _, p := range panels {
		for _, dash := range r.dashboards {
			dashInfo := strings.Split(dash, " ")
			r.panelUrls <- fmt.Sprintf("%s %s%s?panelId=%d&from=%d&to=%d&fullscreen",
				fmt.Sprintf("%s_%s", dashInfo[0], strings.Replace(p.Title, " ", "_", -1)),
				BaseHTTP, dashInfo[1], p.PanelID, FromTime, ToTime)
		}
	}

	return nil
}

func (r *Run) getShareURL(wd selenium.WebDriver, urlInfo string, isPanel bool) error {
	info := strings.Split(urlInfo, " ")
	wd.Get(info[1])
	time.Sleep(2 * time.Second)
	if isPanel {
		elem, err := wd.FindElements(selenium.ByXPATH, existPanelElementFlag)
		if err != nil || len(elem) == 0 {
			return nil
		}
	}
	r.savePng(wd, info[0])

	_, errS := tryLoad(wd, shareIcon, 1, true)
	if errS != nil {
		return errS
	}

	_, errF := tryLoad(wd, snapshotFont, 1, true)
	if errF != nil {
		return errF
	}

	_, errE := tryLoad(wd, externalEnabledButtion, 1, true)
	if errE != nil {
		return errE
	}

	elem, errSE := tryLoad(wd, shareElement, 3, false)
	if errSE != nil {
		return errSE
	}
	et, errT := elem.Text()
	if errT != nil {
		return errT
	}
	fmt.Printf("%s\n%s\n\n", info, et)

	return nil
}

func tryLoad(wd selenium.WebDriver, key string, sleepTime int64, click bool) (selenium.WebElement, error) {
	var elem selenium.WebElement
	var errF error
	for i := 0; i < 3; i++ {
		elem, errF = wd.FindElement(selenium.ByXPATH, key)
		if errF != nil && i == 2 {
			return nil, errF
		} else if errF != nil {
			time.Sleep(time.Duration(sleepTime) * time.Second)
		} else {
			break
		}
	}
	if !click {
		return elem, nil
	}

	for i := 0; i < 3; i++ {
		err := elem.Click()
		if err != nil && i == 2 {
			return nil, err
		} else if err != nil {
			time.Sleep(time.Duration(sleepTime) * time.Second)
		} else {
			break
		}
	}

	return elem, nil

}

func (r *Run) schedule() {
	for wd := range r.wds {
		go func(wd selenium.WebDriver) {
			for {
				select {
				case <-r.ctx.Done():
					return
				default:
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
			}
		}(wd)
	}

}
