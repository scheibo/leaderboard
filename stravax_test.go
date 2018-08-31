package stravax

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

var email = flag.String("email", "", "Email")
var password = flag.String("password", "", "Password")

func TestGetLeaderboardURL(t *testing.T) {
	tests := []struct {
		segmentID int64
		gender    Gender
		filter    Filter
		expected  string
	}{
		{1234, Genders.Male, Filters.Overall,
			"https://www.strava.com/segments/1234?filter=overall&gender=M&per_page=100"},
		{5678, Genders.Female, Filters.Overall,
			"https://www.strava.com/segments/5678?filter=overall&gender=F&per_page=100"},
		{9012, Genders.Male, Filters.CurrentYear,
			"https://www.strava.com/segments/9012?date_range=this_year&filter=current_year&gender=M&per_page=100"},
		{3456, Genders.Female, Filters.CurrentYear,
			"https://www.strava.com/segments/3456?date_range=this_year&filter=current_year&gender=F&per_page=100"},
	}
	for _, tt := range tests {
		actual := getLeaderboardURL(tt.segmentID, tt.gender, tt.filter)
		if actual != tt.expected {
			t.Errorf("getLeaderboardURL(%d, %s, %s): got: %s, want: %s",
				tt.segmentID, tt.gender, tt.filter, actual, tt.expected)
		}
	}
}

func TestGetLeaderboardAndSegment(t *testing.T) {
	expectedSegment := Segment{
		ID:                 2198806,
		Name:               "PCSD",
		Location:           "Dixon, CA",
		Distance:           16110,
		AverageGrade:       0.0008069522036002483,
		ElevationLow:       83,
		ElevationHigh:      96,
		TotalElevationGain: 13,
		MedianElevation:    89.5,
	}
	tests := []struct {
		files                []string
		gender               Gender
		filter               Filter
		expectedRequestCount int64
		expectedLenEntries   int
		expectedEntriesCount int64
	}{
		{[]string{"segment-male-overall.1.html", "segment-male-overall.2.html", "segment-male-overall.3.html", "segment-male-overall.4.html", "segment-male-overall.5.html"},
			Genders.Male, Filters.Overall, 5, 474, 474},
		{[]string{"segment-female-overall.1.html", "segment-female-overall.2.html"}, Genders.Female, Filters.Overall, 2, 120, 120},
		{[]string{"segment-male-yearly.1.html"}, Genders.Male, Filters.CurrentYear, 1, 21, 21},
		{[]string{"segment-female-yearly.1.html"}, Genders.Female, Filters.CurrentYear, 1, 4, 4},
	}
	for _, tt := range tests {
		client := newStubClient(t, tt.files...)
		leaderboard, segment, err := client.GetLeaderboardAndSegment(expectedSegment.ID, tt.gender, tt.filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaderboard.Entries) != tt.expectedLenEntries ||
			leaderboard.EntriesCount != tt.expectedEntriesCount ||
			*segment != expectedSegment ||
			client.RequestCount != tt.expectedRequestCount {
			t.Errorf("GetLeaderboardAndSegment(%d, %s, %s): got: ((%d, %d), %v, %d), want: ((%d, %d), %v, %d)",
				expectedSegment.ID, tt.gender, tt.filter, len(leaderboard.Entries), leaderboard.EntriesCount, *segment,
				client.RequestCount, tt.expectedLenEntries, tt.expectedEntriesCount, expectedSegment, tt.expectedRequestCount)
		}
	}
}

func TestGetLeaderboard(t *testing.T) {
	tests := []struct {
		files                []string
		gender               Gender
		filter               Filter
		expectedRequestCount int64
		expectedLenEntries   int
		expectedEntriesCount int64
	}{
		{[]string{"segment-male-overall.1.html", "segment-male-overall.2.html", "segment-male-overall.3.html", "segment-male-overall.4.html", "segment-male-overall.5.html"},
			Genders.Male, Filters.Overall, 5, 474, 474},
		{[]string{"segment-female-overall.1.html", "segment-female-overall.2.html"}, Genders.Female, Filters.Overall, 2, 120, 120},
		{[]string{"segment-male-yearly.1.html"}, Genders.Male, Filters.CurrentYear, 1, 21, 21},
		{[]string{"segment-female-yearly.1.html"}, Genders.Female, Filters.CurrentYear, 1, 4, 4},
	}
	var segmentID = int64(2198806)
	for _, tt := range tests {
		client := newStubClient(t, tt.files...)
		leaderboard, err := client.GetLeaderboard(segmentID, tt.gender, tt.filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaderboard.Entries) != tt.expectedLenEntries ||
			leaderboard.EntriesCount != tt.expectedEntriesCount ||
			client.RequestCount != tt.expectedRequestCount {
			t.Errorf("GetLeaderboard(%d, %s, %s): got: ((%d, %d), %d), want: ((%d, %d), %d)",
				segmentID, tt.gender, tt.filter, len(leaderboard.Entries), leaderboard.EntriesCount,
				client.RequestCount, tt.expectedLenEntries, tt.expectedEntriesCount, tt.expectedRequestCount)
		}
	}
}

func TestGetLeaderboardPageAndSegment(t *testing.T) {
	expectedSegment := Segment{
		ID:                 2198806,
		Name:               "PCSD",
		Location:           "Dixon, CA",
		Distance:           16110,
		AverageGrade:       0.0008069522036002483,
		ElevationLow:       83,
		ElevationHigh:      96,
		TotalElevationGain: 13,
		MedianElevation:    89.5,
	}
	tests := []struct {
		file                 string
		gender               Gender
		filter               Filter
		page                 int
		expectedRequestCount int64
		expectedLenEntries   int
		expectedEntriesCount int64
	}{
		{"segment-male-overall.4.html", Genders.Male, Filters.Overall, 4, 1, 100, 474},
		{"segment-female-overall.2.html", Genders.Female, Filters.Overall, 2, 1, 20, 120},
		{"segment-male-yearly.1.html", Genders.Male, Filters.CurrentYear, 1, 1, 21, 21},
		{"segment-female-yearly.1.html", Genders.Female, Filters.CurrentYear, 1, 1, 4, 4},
	}
	for _, tt := range tests {
		client := newStubClient(t, tt.file)
		leaderboard, segment, err := client.GetLeaderboardPageAndSegment(expectedSegment.ID, tt.gender, tt.filter, tt.page)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaderboard.Entries) != tt.expectedLenEntries ||
			leaderboard.EntriesCount != tt.expectedEntriesCount ||
			*segment != expectedSegment ||
			client.RequestCount != tt.expectedRequestCount {
			t.Errorf("GetLeaderboardPageAndSegment(%d, %s, %s, %d): got: ((%d, %d), %v, %d), want: ((%d, %d), %v, %d)",
				expectedSegment.ID, tt.gender, tt.filter, tt.page, len(leaderboard.Entries), leaderboard.EntriesCount, *segment,
				client.RequestCount, tt.expectedLenEntries, tt.expectedEntriesCount, expectedSegment, tt.expectedRequestCount)
		}
	}
}

func TestGetLeaderboardPage(t *testing.T) {
	tests := []struct {
		file                 string
		gender               Gender
		filter               Filter
		page                 int
		expectedRequestCount int64
		expectedLenEntries   int
		expectedEntriesCount int64
	}{
		{"segment-male-overall.4.html", Genders.Male, Filters.Overall, 4, 1, 100, 474},
		{"segment-female-overall.2.html", Genders.Female, Filters.Overall, 2, 1, 20, 120},
		{"segment-male-yearly.1.html", Genders.Male, Filters.CurrentYear, 1, 1, 21, 21},
		{"segment-female-yearly.1.html", Genders.Female, Filters.CurrentYear, 1, 1, 4, 4},
	}
	var segmentID = int64(2198806)
	for _, tt := range tests {
		client := newStubClient(t, tt.file)
		leaderboard, err := client.GetLeaderboardPage(segmentID, tt.gender, tt.filter, tt.page)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaderboard.Entries) != tt.expectedLenEntries ||
			leaderboard.EntriesCount != tt.expectedEntriesCount ||
			client.RequestCount != tt.expectedRequestCount {
			t.Errorf("GetLeaderboardPage(%d, %s, %s, %d): got: ((%d, %d), %d), want: ((%d, %d), %d)",
				segmentID, tt.gender, tt.filter, tt.page, len(leaderboard.Entries), leaderboard.EntriesCount,
				client.RequestCount, tt.expectedLenEntries, tt.expectedEntriesCount, tt.expectedRequestCount)
		}
	}
}

func TestUpdateGolden(t *testing.T) {
	if *email == "" || *password == "" {
		return
	}
	client, err := NewClient(*email, *password)
	if err != nil {
		t.Fatal(err)
	}

	fixtures := []struct {
		prefix   string
		gender   Gender
		filter   Filter
		requests int
	}{
		{"segment-male-overall", Genders.Male, Filters.Overall, 5},
		{"segment-female-overall", Genders.Female, Filters.Overall, 2},
		{"segment-male-yearly", Genders.Male, Filters.CurrentYear, 1},
		{"segment-female-yearly", Genders.Female, Filters.CurrentYear, 1},
	}
	for _, fix := range fixtures {
		url := getLeaderboardURL(2198806, fix.gender, fix.filter)
		for i := 0; i < fix.requests; i++ {
			resp, err := client.httpClient.Get(fmt.Sprintf("%s&page=%d", url, i+1))
			if err != nil {
				t.Fatal(err)
			}

			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("bad response code %d", resp.StatusCode)
			}
			file := fmt.Sprintf("%s.%d.html", fix.prefix, i+1)
			bytes, err := ioutil.ReadAll(resp.Body)
			err = ioutil.WriteFile(filepath.Join("testdata", file), bytes, 0600)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

}

func newStubClient(t *testing.T, files ...string) *Client {
	var contents []string
	for _, file := range files {
		content, err := ioutil.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, string(content))
	}
	return NewStubClient(contents...)
}
