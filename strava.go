//package strava
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/strava/go.strava"

	"golang.org/x/net/publicsuffix"
)

const USER_AGENT = "strava-leaderboard/0.0.1"

const QPS = 10

// MAX_PER_PAGE is the maximum number of entires which can be requested per page.
// NOTE: This is 100 when using the API, but for some reason 100 is the limit when
// scraping.
const MAX_PER_PAGE = 100

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
	URL    string `json:"url"`
	Name   string `json:"name"`
	Gender Gender `json:"gender"`
}

// TODO not going to be as accurate as the API!
type Segment struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Location           string  `json:"location"`
	Distance           float64 `json:"distance"`
	AverageGrade       float64 `json:"average_grade"`
	ElevationLow       float64 `json:"elevation_low"`
	ElevationHigh      float64 `json:"elevation_high"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
}

type LeaderboardEntry struct {
	Rank        int64     `json:"rank"`
	Athlete     Athlete   `json:"athlete"`
	EffortID    int64     `json:"effort_id"`
	StartDate   time.Time `json:"start_date"`
	ElapsedTime int64     `json:"elapsed_time"`
}

type Leaderboard struct {
	Segment *Segment            `json:"segment"`
	Entries []*LeaderboardEntry `json:"entries"`
}

type Client struct {
	throttle     <-chan time.Time
	httpClient   *http.Client
	stravaClient *strava.Client
}

type transport struct{}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", USER_AGENT)
	return http.DefaultTransport.RoundTrip(req)
}

func NewClient(email, password string, accessToken ...string) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Jar:       jar,
		Timeout:   10 * time.Second,
		Transport: &transport{},
	}
	c := &Client{
		throttle:   time.Tick(QPS),
		httpClient: httpClient,
	}
	if len(accessToken) > 0 && accessToken[0] != "" {
		c.stravaClient = strava.NewClient(accessToken[0])
	}

	resp, err := c.httpClient.Get("https://www.strava.com/login")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	csrf_param, ok := doc.Find("meta[name=csrf-param]").Attr("content")
	if !ok {
		return nil, errors.New("Could not find csrf-param")
	}
	csrf_token, ok := doc.Find("meta[name=csrf-token]").Attr("content")
	if !ok {
		return nil, errors.New("Could not find csrf-token")
	}

	resp, err = c.httpClient.PostForm(
		"https://www.strava.com/session",
		url.Values{
			"email":       {email},
			"password":    {password},
			"remember_me": {"on"},
			csrf_param:    {csrf_token}})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	doc, err = goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	if doc.Find("title").Text() != "Dashboard | Strava" {
		return nil, errors.New("Login was unsuccessful!")
	}

	return c, nil
}

type stubResponseTransport struct {
	http.Transport
	content    string
	statusCode int
}

func DUMP(resp *http.Response) {
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", dump)
}

func HTML(s *goquery.Selection) {
	html, err := goquery.OuterHtml(s)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", html)
}

func (t *stubResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Status:     http.StatusText(t.statusCode),
		StatusCode: t.statusCode,
	}
	resp.Body = ioutil.NopCloser(strings.NewReader(t.content))

	return resp, nil
}

func NewStubClient(content string, statusCode ...int) *Client {
	c := &Client{}
	t := &stubResponseTransport{content: content}

	if len(statusCode) != 0 {
		t.statusCode = statusCode[0]
	}

	c.httpClient = &http.Client{Transport: t}
	return c
}

func (c *Client) GetLeaderboard(segmentId int64, gender Gender, filter Filter) (*Leaderboard, int64, error) {
	url := fmt.Sprintf("https://www.strava.com/segments/%d?", segmentId)
	// Strava doesn't respect current_year properly without a date_range
	if filter == Filters.CurrentYear {
		url = fmt.Sprintf("%sdate_range=this_year&", url)
	}
	url = fmt.Sprintf("%sfilter=%s&gender=%s&per_page=%d", url, filter, gender, MAX_PER_PAGE)

	var reqs, pages, api int64
	reqs = 1
	<-c.throttle // rate limiting
	resp, err := c.httpClient.Get(fmt.Sprintf("%s&page=%d", url, reqs))
	if err != nil {
		return nil, reqs, err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, reqs, err
	}

	p := doc.Find(".pagination li:nth-last-child(2)").Text()
	if p == "" {
		pages = 1
	} else {
		pages, err = parseInt(p)
		if err != nil {
			return nil, reqs, err
		}
	}

	leaderboard := &Leaderboard{}
	if c.stravaClient != nil {
		<-c.throttle // rate limiting
		leaderboard.Segment, err = getSegment(c.stravaClient, segmentId)
		api = 1
	} else {
		leaderboard.Segment, err = parseSegment(doc, segmentId)
	}
	if err != nil {
		return nil, reqs + api, err
	}
	leaderboard.Entries, err = addToLeaderboard(doc, gender, leaderboard.Entries)
	if err != nil {
		return nil, reqs + api, err
	}

	for ; reqs <= pages; reqs++ {
		<-c.throttle // rate limiting
		resp, err := c.httpClient.Get(fmt.Sprintf("%s&page=%d", url, reqs))
		if err != nil {
			return nil, reqs + api, err
		}

		defer resp.Body.Close()
		doc, err := goquery.NewDocumentFromReader(io.Reader(resp.Body))
		if err != nil {
			return nil, reqs + api, err
		}

		leaderboard.Entries, err = addToLeaderboard(doc, gender, leaderboard.Entries)
		if err != nil {
			return nil, reqs + api, err
		}
	}

	return leaderboard, reqs + api, nil
}

func getSegment(stravaClient *strava.Client, segmentId int64) (*Segment, error) {
	segment, err := strava.NewSegmentsService(stravaClient).Get(segmentId).Do()
	if err != nil {
		return nil, err
	}

	s := &Segment{ID: segmentId}
	s.Name = segment.Name
	s.Location = fmt.Sprintf("%s, %s", segment.City, segment.State)
	s.Distance = segment.Distance
	s.ElevationLow = segment.ElevationLow
	s.ElevationHigh = segment.ElevationHigh

	gain := s.ElevationHigh - s.ElevationLow
	if segment.TotalElevationGain > gain {
		s.TotalElevationGain = segment.TotalElevationGain
	} else {
		s.TotalElevationGain = gain
	}
	s.AverageGrade = s.TotalElevationGain / s.Distance * 100.0

	return s, nil
}

func parseSegment(doc *goquery.Document, segmentId int64) (*Segment, error) {
	s := &Segment{ID: segmentId}

	div := doc.Find(".segment-heading").First()
	name, ok := div.Find(".segment-name span[data-full-name]").Attr("data-full-name")
	if !ok {
		return nil, errors.New("Could not find segment name!")
	}
	s.Name = name
	s.Location = strings.TrimSpace(div.Find(".location").Contents().Not("strong").Text())

	stats := div.Find(".stat-text")

	val, err := parseFloat(stats.Eq(0).Contents().Not("abbr").Text())
	if err != nil {
		return nil, err
	}
	s.Distance = val * 1000

	val, err = parseFloat(stats.Eq(2).Contents().Not("abbr").Text())
	if err != nil {
		return nil, err
	}
	s.ElevationLow = val

	val, err = parseFloat(stats.Eq(3).Contents().Not("abbr").Text())
	if err != nil {
		return nil, err
	}
	s.ElevationHigh = val

	val, err = parseFloat(stats.Eq(4).Contents().Not("abbr").Text())
	if err != nil {
		return nil, err
	}

	gain := s.ElevationHigh - s.ElevationLow
	if val > gain {
		s.TotalElevationGain = val
	} else {
		s.TotalElevationGain = gain
	}
	s.AverageGrade = s.TotalElevationGain / s.Distance * 100.0

	return s, nil
}

func addToLeaderboard(doc *goquery.Document, gender Gender, entries []*LeaderboardEntry) ([]*LeaderboardEntry, error) {
	var err error
	doc.Find(".table-leaderboard tbody tr").EachWithBreak(func(i int, tr *goquery.Selection) bool {
		tds := tr.Find("td")
		entry := new(LeaderboardEntry)

		r := strings.TrimSpace(tds.Eq(0).Text())
		if r == "" {
			entry.Rank = 1
		} else {
			entry.Rank, err = parseInt(r)
			if err != nil {
				return false
			}
		}

		td := tds.Eq(1)
		href, ok := td.Find("a").Attr("href")
		if !ok {
			err = errors.New("Could not find athlete URL!")
			return false
		}
		url := fmt.Sprintf("https://www.strava.com%s", href)
		entry.Athlete = Athlete{URL: url, Name: strings.TrimSpace(td.Text()), Gender: gender}

		td = tds.Eq(2)
		entry.StartDate, err = time.Parse("Jan 2, 2006", strings.TrimSpace(td.Text()))
		if err != nil {
			return false
		}
		href, ok = td.Find("a").Attr("href")
		if !ok {
			err = errors.New("Could not find effort ID!")
			return false
		}
		var id int64
		id, err = parseInt(strings.TrimPrefix(href, "/segment_efforts/"))
		if err != nil {
			return false
		}
		entry.EffortID = id

		entry.ElapsedTime, _ = parseElapsedTime(strings.TrimSpace(tds.Eq(7).Text()))

		entries = append(entries, entry)
		return true
	})

	if err != nil {
		return nil, err
	}
	return entries, nil
}

func parseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 0)
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
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

// go run strava.go -email=$STRAVA_EMAIL -password=$STRAVA_PASSWORD -token=$STRAVA_ACCESS_TOKEN -id=8109834
func main() {
	var email, password, accessToken string
	var segmentId int64

	flag.StringVar(&email, "email", "", "Email")
	flag.StringVar(&password, "password", "", "Password")
	flag.StringVar(&accessToken, "token", "", "Access Token")
	flag.Int64Var(&segmentId, "id", -1, "Segment Id")

	flag.Parse()

	if email == "" {
		log.Fatal("Please provide an email")
	}
	if password == "" {
		log.Fatal("Please provide a password")
	}
	if segmentId < 0 {
		log.Fatal("Please provide a segment")
	}

	//content, err := ioutil.ReadFile("foo.html")
	//if err != nil {
	//log.Fatal(err)
	//}

	//client := NewStubClient(string(content), 200)
	client, err := NewClient(email, password, accessToken)
	if err != nil {
		log.Fatal(err)
	}

	leaderboard, _, err := client.GetLeaderboard(segmentId, Genders.Female, Filters.CurrentYear)
	if err != nil {
		log.Fatal(err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.Encode(leaderboard)
}
