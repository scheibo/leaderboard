//package strava
package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Gender string

var Genders = struct {
	Unspecified Gender
	Male        Gender
	Female      Gender
}{"", "M", "F"}

type Filter string

var Filters = struct {
	Overall     Filter
	CurrentYear Filter
}{"overall", "current_year"}

type Athlete struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Gender Gender `json:"gender"`
}

type Segment struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Location           string  `json:"location"`
	Distance           int64   `json:"distance"`
	AverageGrade       float64 `json:average_grade"`
	ElevationLow       float64 `json:"elevation_low"`
	ElevationHigh      float64 `json:"elevation_high"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
}

type LeaderboardEntry struct {
	Rank        int64       `json:"rank"`
	Athlete     Athlete   `json:"athlete"`
	EffortID    int64     `json:"effort_id"`
	StartDate   time.Time `json:"start_date"`
	ElapsedTime int64       `json:"elapsed_time"`
}

type Leaderboards struct {
	Segment           Segment             `json:"segment"`
	MaleOverall       []*LeaderboardEntry `json:"male_overall"`
	MaleCurrentYear   []*LeaderboardEntry `json:"male_current_year"`
	FemaleOverall     []*LeaderboardEntry `json:"female_overall"`
	FemaleCurrentYear []*LeaderboardEntry `json:"female_current_year"`
}

// MAX_PER_PAGE is the maximum number of entires which can be requested per page.
// NOTE: This is 100 when using the API, but for some reason 100 is the limit when
// scraping.
//const MAX_PER_PAGE = 100

//func pageURL(segmentId int64, gender Gender, filter Filter, page string) string {
//return "https://www.strava.com/segments/" + c.service.id +
//"?filter=" + filter +
//"&gender=" + gender +
//"&per_page=" + MAX_PER_PAGE +
//"&page=" + page
//}

//func GetLeaderboards(email, password string, segmentId int64) (*Leaderboards, error) {

//}

//const VERSION = '0.0.1'
//const USER_AGENT = "strava-leaderbaord/" + VERSION

//func login(email, password string) Foo, error {

//}

func parseInt(s string) (int64, error) {
	log.Print(s)
	return strconv.ParseInt(s, 10, 0)
}

func main() {
	raw, err := os.Open("foo.html")
	if err != nil {
		log.Fatal(err)
	}
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		log.Fatal(err)
	}

	// TODO really only one page
	leaderboard := []*LeaderboardEntry{}

	// TODO handle errors!

	// Find the review items
	doc.Find(".table-leaderboard tbody tr").Each(func(i int, tr *goquery.Selection) {
		// For each item found, get the band and title
		tds := tr.Find("td")
		entry := new(LeaderboardEntry)
		entry.Rank, _ = parseInt(strings.TrimSpace(tds.Eq(0).Text()))
		td := tds.Eq(1)
		href, _ := td.Find("a").Attr("href")
		id, _ := parseInt(strings.TrimPrefix(href, "/athletes/"))
		entry.Athlete = Athlete{ ID: id, Name: strings.TrimSpace(td.Text()), Gender: Genders.Female }
		td = tds.Eq(2)
		entry.StartDate, _ = time.Parse("Jan 02, 2006", strings.TrimSpace(td.Text()))
		href, _ = td.Find("a").Attr("href")
		id, _ = parseInt(strings.TrimPrefix(href, "/segment_efforts/"))
		entry.EffortID = id
		entry.ElapsedTime, _ = parseElapsedTime(strings.TrimSpace(tds.Eq(7).Text()))

		leaderboard = append(leaderboard, entry)
	})

	enc := json.NewEncoder(os.Stdout)
	enc.Encode(leaderboard[0])
}

func parseElapsedTime(str string) (int64, error) {
	var x string
	var h, m, s int64
	var err error

	a := strings.Split(str, ":")

	if len(a) == 3 {
		x, a = a[0], a[1:]
		h, err = parseInt(x)
		if err != nil {
			return 0, err
		}
	}
	if len(a) == 2 {
		x, a = a[0], a[1:]
		m, err = parseInt(x)
		if err != nil {
			return 0, err
		}
	}
	s, err = parseInt(strings.TrimSuffix(a[0], "s"))
	if err != nil {
		return 0, err
	}
	return h*3600 + m*60 + s, nil
}
