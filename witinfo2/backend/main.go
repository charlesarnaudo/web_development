package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/resty.v1"
)

//Schedule is the schedule struct for the folder
type Schedule map[int][]string

//LoginForm handles all the login transactions
type LoginForm struct {
	Email    string
	Password string
}

//VersionStruct is whats returned to get version info
type VersionStruct struct {
	BuildVersion string
	BuildTime    string
	Uptime       string
}

var (
	staticfolder string
	port         string
	//BuildVersion is
	BuildVersion = "local"
	//BuildTime is when this was built
	BuildTime = ""
	//Uptime is when server was started
	Uptime time.Time
)

func main() {
	Uptime = time.Now()
	if BuildTime == "" {
		BuildTime = Uptime.String()
	}
	flag.StringVar(&staticfolder, "StaticFiles", "../website/build", "Location of static files")
	flag.StringVar(&port, "port", ":9009", "Port for webserver")
	flag.Parse()
	router := httprouter.New()
	router.GET("/charties", handleCharties)
	router.POST("/schedule", handleSchedule)
	router.POST("/moolah", handleMoolah)
	router.GET("/version", handleVersion)
	router.NotFound = http.FileServer(http.Dir(staticfolder))
	log.Printf("BuildTime: %s\tBuildVersion: %s", BuildTime, BuildVersion)
	log.Printf("Starting server on %s with files from %s", staticfolder, port)
	log.Fatal(http.ListenAndServe(port, router))
}
func handleVersion(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var vstruct VersionStruct
	vstruct.Uptime = time.Since(Uptime).String()
	vstruct.BuildTime = BuildTime
	vstruct.BuildVersion = BuildVersion
	bytes, berr := json.Marshal(vstruct)
	if berr == nil {
		w.Write(bytes)
	} else {
		http.Error(w, "Failed to get version", http.StatusInternalServerError)
	}
}
func handleCharties(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	data, dataerr := chartwellsOpenLocations()
	if dataerr == nil {
		w.Write(data)
	} else {
		http.Error(w, fmt.Sprintf("Failed to get charties: %s", dataerr), http.StatusInternalServerError)
	}
}

func handleSchedule(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logindecoder := json.NewDecoder(r.Body)
	var lgnstruct LoginForm
	decodeerr := logindecoder.Decode(&lgnstruct)
	if decodeerr != nil {
		log.Printf("whoops, %s", decodeerr)
		http.Error(w, "whatever", 500)
		return
	}
	schedule, scherr := currentSchedule(lgnstruct.Email, lgnstruct.Password)
	for i := 5; i >= 0; i-- {
		schedule[i+1], schedule[i] = schedule[i], schedule[i+1]
	}
	encoder := json.NewEncoder(w)
	encerr := encoder.Encode(schedule)
	if scherr != nil {
		log.Printf("whoops, %s", scherr)

	}
	if encerr != nil {
		log.Printf("whoops, %s", encerr)
	}
}

//handleMoolah gets money
func handleMoolah(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logindecoder := json.NewDecoder(r.Body)
	var lgnstruct LoginForm
	decodeerr := logindecoder.Decode(&lgnstruct)
	if decodeerr != nil {
		log.Printf("whoops, %s", decodeerr)
		http.Error(w, "whatever", 500)
		return
	}
	schedule, scherr := fenwayCashBalance(lgnstruct.Email, lgnstruct.Password)
	if scherr != nil {
		http.Error(w, fmt.Sprintf("whoops, %s", scherr), http.StatusInternalServerError)
	}
	encoder := json.NewEncoder(w)
	encerr := encoder.Encode(schedule)

	if encerr != nil {
		log.Printf("whoops, %s", encerr)
	}
}

//
func chartwellsOpenLocations() ([]byte, error) {
	url := "https://api.dineoncampus.com/v1/locations/open"
	timestamp := time.Now().UTC().Format(time.RFC3339)
	//2019-04-10T19:48:11.346Z
	params := map[string]string{
		"site_id":   "5751fd3390975b60e048938a",
		"timestamp": timestamp,
	}
	r, err := resty.R().SetQueryParams(params).Get(url)
	return r.Body(), err
}

func currentSchedule(username string, password string) (Schedule, error) {
	sess := resty.New()

	// Load the login page
	url := "https://cas.wit.edu/cas/login"
	params := map[string]string{
		"service": "https://prodweb2.wit.edu:443/ssomanager/c/SSB",
	}
	r, err := sess.NewRequest().SetQueryParams(params).Get(url)
	if err != nil {
		return Schedule{}, err
	}

	// Parse the form in the response for hidden values that must be included in the auth request
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body()))
	if err != nil {
		return Schedule{}, err
	}
	form := doc.Find("form").First()
	path, _ := form.Attr("action")
	url = "https://cas.wit.edu" + path
	data := make(map[string]string)
	form.Find("form input[type=hidden]").Each(func(i int, s *goquery.Selection) {
		data[s.AttrOr("name", "")] = s.AttrOr("value", "")
	})
	data["username"] = username
	data["password"] = password

	// Authenticate
	r, err = sess.NewRequest().SetFormData(data).Post(url)
	if err != nil && !strings.HasSuffix(err.Error(), "auto redirect is disabled") {
		return Schedule{}, err
	}
	if r.StatusCode() != 302 {
		return Schedule{}, fmt.Errorf("Incorrect credentials")
	}

	// Apply authentication credentials
	//fmt.Println(r.Header().Get("Location"))
	r, err = sess.NewRequest().Get(r.Header().Get("Location"))
	if err != nil && !strings.HasSuffix(err.Error(), "auto redirect is disabled") {
		return Schedule{}, err
	}

	// Get schedule
	r, err = sess.NewRequest().Get("https://prodweb2.wit.edu/SSBPROD/bwskfshd.P_CrseSchd")

	// Parse schedule into something more bite-sized
	doc, err = goquery.NewDocumentFromReader(bytes.NewReader(r.Body()))
	if err != nil {
		return Schedule{}, err
	}
	sched := make(map[int][]string)
	for i := 0; i < 7; i++ {
		sched[i] = []string{}
	}
	colStatus := make([]int, 7)
	doc.Find("table.datadisplaytable tr").Each(func(rowIdx int, rowEl *goquery.Selection) {
		for colStatusIdx := range colStatus {
			if colStatus[colStatusIdx] > 0 {
				colStatus[colStatusIdx]--
			}
		}
		cellEls := rowEl.Find("td").Nodes
		addend := 0
		for colIdx, cellEl := range cellEls {
			for colStatus[colIdx+addend] > 0 {
				addend++
			}

			rowspan, isFilled := goquery.NewDocumentFromNode(cellEl).Attr("rowspan")
			if !isFilled {
				continue
			}
			cellElStr, _ := goquery.NewDocumentFromNode(cellEl).Children().First().Html()
			cellElStr = strings.ReplaceAll(cellElStr, "<br/>", "\n")
			sched[colIdx+addend] = append(sched[colIdx+addend], cellElStr)
			rowspanInt, _ := strconv.Atoi(rowspan)
			colStatus[colIdx+addend] = rowspanInt
			//fmt.Println(colIdx+addend, colStatus)
		}
	})
	return sched, nil
}

func fenwayCashBalance(username string, password string) (float64, error) {
	sess := resty.New()

	// Load the login page. TODO: generalize "emmanuel", "massart", "mcphs", "simmons", "wit"
	url := "https://wit.campuscardcenter.com/ch/login.html"
	r, err := sess.NewRequest().Get(url)
	if err != nil {
		return 0, err
	}

	// Parse the form in the response for hidden values that must be included in the auth request
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body()))
	if err != nil {
		return 0, err
	}
	form := doc.Find("form")
	path, _ := form.Attr("action")
	url = "https://wit.campuscardcenter.com" + path
	data := make(map[string]string)
	doc.Find("input[type=hidden]").Each(func(i int, s *goquery.Selection) {
		data[s.AttrOr("name", "")] = s.AttrOr("value", "")
	})
	data["action"] = "Login"
	data["username"] = username
	data["password"] = password

	// Authenticate
	r, err = sess.NewRequest().SetFormData(data).Post(url)
	if err != nil && !strings.HasSuffix(err.Error(), "auto redirect is disabled") {
		return 0, err
	}
	if r.StatusCode() != 302 {
		return 0, fmt.Errorf("Incorrect credentials")
	}

	// Fetch the main Fenway Cash cardholder page
	r, err = sess.NewRequest().Get("https://wit.campuscardcenter.com/ch/")
	if err != nil {
		return 0, err
	}
	rBody := r.Body()

	// Parse out Fenway Cash balance
	doc, err = goquery.NewDocumentFromReader(bytes.NewReader(rBody))
	if err != nil {
		return 0, err
	}
	value, _ := doc.Find("div[align=right]").First().Html()
	value = strings.Trim(value, "$ ")
	valueFloat, _ := strconv.ParseFloat(value, 64)
	return valueFloat, nil
}
