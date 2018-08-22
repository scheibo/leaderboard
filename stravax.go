// Package stravax extends to Strava API to allow for retrieving complete Strava
// leaderboard information for a logged in user.
package stravax

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/strava/go.strava"

	"golang.org/x/net/publicsuffix"
)

// USER_AGENT is the user agent we will use when making requests against the frontend.
const USER_AGENT = "stravax/0.0.1"

// QPS_LIMIT is the maximum number of requests we will make in a second to both
// the API and the frontend combined.
const QPS_LIMIT = 10

// MAX_PER_PAGE is the maximum number of entries which can be requested per page.
// NOTE: This is 100 when using the API, but for some reason 100 is the limit
// when scraping the frontend.
const MAX_PER_PAGE = 100

// Gender is the gender of the athlete.
type Gender string

// Genders represents all possible genders Strava supports.
var Genders = struct {
	Unspecified Gender
	Male        Gender
	Female      Gender
}{"", "M", "F"}

// Filter is the filter used on the leaderboard.
type Filter string

// Filters represents the Strava filters this client supports.
var Filters = struct {
	Overall     Filter
	CurrentYear Filter
}{"overall", "current_year"}

// Athlete holds information about a Strava athlete required to render a leaderboard.
type Athlete struct {
	URL    string `json:"url"`
	Name   string `json:"name"`
	Gender Gender `json:"gender"`
}

// Segment contains the Strava segment details.
// NOTE: The segment information stored in the frontend leaderboard page is
// inherently less accurate than the information from the API.
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

// LeaderboardEntry is a single entry in a leaderboard, representing the best
// effort on a segment by a particular athlete.
type LeaderboardEntry struct {
	Rank        int64     `json:"rank"`
	Athlete     Athlete   `json:"athlete"`
	EffortID    int64     `json:"effort_id"`
	StartDate   time.Time `json:"start_date"`
	ElapsedTime int64     `json:"elapsed_time"`
}

// Leaderboard contains LeaderboardEntry objects sorted by their rank
// according to Strava. len(Entries) may not equal EntriesCount if the
// Leaderboard has not been completely fetched or entries were added or
// removed from the leaderboard during fetching.
type Leaderboard struct {
	Entries      []*LeaderboardEntry `json:"entries"`
	EntriesCount int64               `json:"entries_count"`
}

// Client is used to retrieve Segment and Leaderboard information from the
// Strava API and frontend. Calls to Strava are rate limiting to QPS_LIMIT
// requests/second, and the number of requests issued is tracked by
// RequestCount.
type Client struct {
	RequestCount int64
	throttle     <-chan time.Time
	httpClient   *http.Client
	stravaClient *strava.Client
}

type transport struct{}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", USER_AGENT)
	return http.DefaultTransport.RoundTrip(req)
}

// NewClient returns an authenticated Client for querying Strava.
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
		throttle:   time.Tick(QPS_LIMIT),
		httpClient: httpClient,
	}
	if len(accessToken) > 0 && accessToken[0] != "" {
		c.stravaClient = strava.NewClient(accessToken[0])
	}

	return c.login(email, password)
}

func (c *Client) login(email, password string) (*Client, error) {
	resp, err := c.httpClient.Get("https://www.strava.com/login")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	csrfParam, ok := doc.Find("meta[name=csrf-param]").Attr("content")
	if !ok {
		return nil, errors.New("could not find csrf-param")
	}
	csrfToken, ok := doc.Find("meta[name=csrf-token]").Attr("content")
	if !ok {
		return nil, errors.New("could not find csrf-token")
	}

	resp, err = c.httpClient.PostForm(
		"https://www.strava.com/session",
		url.Values{
			"email":       {email},
			"password":    {password},
			"remember_me": {"on"},
			csrfParam:     {csrfToken}})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	doc, err = goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	if doc.Find("title").Text() != "Dashboard | Strava" {
		return nil, errors.New("login was unsuccessful")
	}

	return c, nil
}

type stubResponseTransport struct {
	http.Transport
	content []string
	reqs    int64
}

func (t *stubResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	statusCode := 200
	resp := &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
	}
	resp.Body = ioutil.NopCloser(strings.NewReader(t.content[t.reqs]))
	t.reqs++

	return resp, nil
}

// NewStubClient returns content for each subsequent request that is made.
func NewStubClient(content ...string) *Client {
	c := &Client{}
	c.httpClient = &http.Client{Transport: &stubResponseTransport{content: content}}
	return c
}

// GetSegment returns the data for the segment identified by segmentID using the Strava API.
func (c *Client) GetSegment(segmentID int64) (*Segment, error) {
	c.request()
	segment, err := strava.NewSegmentsService(c.stravaClient).Get(segmentID).Do()
	if err != nil {
		return nil, err
	}

	s := &Segment{ID: segmentID}
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

// GetLeaderboardAndSegment returns the leaderboard of segmentID for the specified gender
// and filter as well the segment details.
func (c *Client) GetLeaderboardAndSegment(segmentID int64, gender Gender, filter Filter) (*Leaderboard, *Segment, error) {
	return c.getLeaderboard(segmentID, gender, filter, true)
}

// GetLeaderboard returns the leaderboard of segmentID for the specified gender and filter.
func (c *Client) GetLeaderboard(segmentID int64, gender Gender, filter Filter) (*Leaderboard, error) {
	leaderboard, _, err := c.getLeaderboard(segmentID, gender, filter, false)
	return leaderboard, err
}

// GetLeaderboardPageAndSegment returns the specified page of the leaderboard for segmentID for
// given gender and filter as well as the segment details.
func (c *Client) GetLeaderboardPageAndSegment(segmentID int64, gender Gender, filter Filter, page int) (*Leaderboard, *Segment, error) {
	leaderboard, segment, _, err := c.getLeaderboardPageForURL(getLeaderboardURL(segmentID, gender, filter), gender, page, true)
	return leaderboard, segment, err
}

// GetLeaderboardPage returns the specified page of the leaderboard for segmentID for given gender and filter.
func (c *Client) GetLeaderboardPage(segmentID int64, gender Gender, filter Filter, page int) (*Leaderboard, error) {
	leaderboard, _, _, err := c.getLeaderboardPageForURL(getLeaderboardURL(segmentID, gender, filter), gender, page, false)
	return leaderboard, err
}

func (c *Client) getLeaderboard(segmentID int64, gender Gender, filter Filter, includeSegment bool) (*Leaderboard, *Segment, error) {
	var next *Leaderboard
	url := getLeaderboardURL(segmentID, gender, filter)

	page := 1
	leaderboard, segment, final, err :=
		c.getLeaderboardPageForURL(url, gender, page, includeSegment)
	if err != nil {
		return nil, nil, err
	}

	for ; !final; page++ {
		next, _, final, err =
			c.getLeaderboardPageForURL(
				url, gender, page, false)
		if err != nil {
			return nil, nil, err
		}
		// NOTE: EntriesCount could change if new activities are uploaded or
		// deleted during fetching, the next fetched count always takes precedence.
		leaderboard.EntriesCount = next.EntriesCount
		leaderboard.Entries = append(leaderboard.Entries, next.Entries...)
	}

	return leaderboard, segment, nil
}

func (c *Client) getLeaderboardPageForURL(url string, gender Gender, page int, includeSegment bool) (*Leaderboard, *Segment, bool, error) {
	var leaderboard *Leaderboard
	var segment *Segment
	var final bool
	var err error

	c.request()
	resp, err := c.httpClient.Get(fmt.Sprintf("%s&page=%d", url, page))
	if err != nil {
		return nil, nil, false, err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(io.Reader(resp.Body))
	if err != nil {
		return nil, nil, false, err
	}

	if includeSegment {
		segment, err = parseSegment(doc)
		if err != nil {
			return nil, nil, false, err
		}
	}
	leaderboard, err = parseLeaderboard(doc, gender)
	if err != nil {
		return nil, nil, false, err
	}

	// When building up the results, we can't simply loop until len(leaderboard.Entries)
	// == leaderboard.EntryCount  because that's not guaranteed to ever be true. We can
	// definitely stop looping once Strava is no longer returning entries, and we can
	// also use the pagination information to be fairly confident we should stop.
	final = len(leaderboard.Entries) == 0 || isFinalPage(doc)

	return leaderboard, segment, final, nil
}

func (c *Client) request() {
	if c.throttle != nil {
		<-c.throttle // rate limiting
	}
	c.RequestCount++
}

func getLeaderboardURL(segmentID int64, gender Gender, filter Filter) string {
	url := fmt.Sprintf("https://www.strava.com/segments/%d?", segmentID)
	// Strava doesn't respect current_year properly without a date_range
	if filter == Filters.CurrentYear {
		url = fmt.Sprintf("%sdate_range=this_year&", url)
	}
	return fmt.Sprintf(
		"%sfilter=%s&gender=%s&per_page=%d", url, filter, gender, MAX_PER_PAGE)
}

func parseSegment(doc *goquery.Document) (*Segment, error) {
	s := &Segment{}
	attr, ok := doc.Find(".segment-name button").Attr("data-segment-id")
	if !ok {
		return nil, errors.New("could not find segment ID")
	}
	id, err := parseInt(attr)
	if err != nil {
		return nil, err
	}
	s.ID = id

	div := doc.Find(".segment-heading").First()
	name, ok := div.Find(".segment-name span[data-full-name]").Attr("data-full-name")
	if !ok {
		return nil, errors.New("could not find segment name")
	}
	s.Name = name
	s.Location = strings.TrimSpace(
		div.Find(".location").Contents().Not("strong").Text())

	stats := div.Find(".stat-text")

	val, err := parseStat(stats, 0)
	if err != nil {
		return nil, err
	}
	s.Distance = val * 1000

	val, err = parseStat(stats, 2)
	if err != nil {
		return nil, err
	}
	s.ElevationLow = val

	val, err = parseStat(stats, 3)
	if err != nil {
		return nil, err
	}
	s.ElevationHigh = val

	val, err = parseStat(stats, 4)
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

func parseStat(s *goquery.Selection, i int) (float64, error) {
	return parseFloat(s.Eq(i).Contents().Not("abbr").Text())
}

func parseLeaderboard(doc *goquery.Document, gender Gender) (*Leaderboard, error) {
	var leaderboard Leaderboard
	split := strings.Split(doc.Find(".standing").Text(), "/")
	val, err := parseInt(strings.TrimSpace(split[len(split)-1]))
	if err != nil {
		return nil, err
	}
	leaderboard.EntriesCount = val

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
			err = errors.New("could not find athlete URL")
			return false
		}
		url := fmt.Sprintf("https://www.strava.com%s", href)
		entry.Athlete = Athlete{
			URL:    url,
			Name:   strings.TrimSpace(td.Text()),
			Gender: gender,
		}
		td = tds.Eq(2)
		entry.StartDate, err =
			time.Parse("Jan 2, 2006", strings.TrimSpace(td.Text()))
		if err != nil {
			return false
		}
		href, ok = td.Find("a").Attr("href")
		if !ok {
			err = errors.New("could not find effort ID")
			return false
		}
		var id int64
		id, err = parseInt(strings.TrimPrefix(href, "/segment_efforts/"))
		if err != nil {
			return false
		}
		entry.EffortID = id

		entry.ElapsedTime, _ =
			parseElapsedTime(strings.TrimSpace(tds.Eq(7).Text()))

		leaderboard.Entries = append(leaderboard.Entries, entry)
		return true
	})

	if err != nil {
		return nil, err
	}
	return &leaderboard, nil
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

func isFinalPage(doc *goquery.Document) bool {
	// TODO(kjs): should this be '&&' instead?
	return doc.Find(".pagination li:nth-last-child(2)").HasClass("active") ||
		doc.Find(".pagination li.next_page").HasClass("disabled")
}
