package leaderboard

import (
	"testing"
)

func TestGetLeaderboardUrl(t *testing.T) {
	tests := []struct {
		segmentId int64
		gender    Gender
		filter    Filter
		expected  string
	}{
		{1234, Genders.Male, Filters.Overall,
			"https://www.strava.com/segments/1234?filter=overall&gender=M&per_page=100"},
		{5678, Genders.Female, Filters.Overall,
			"https://www.strava.com/segments/5678?filter=overall&gender=F&per_page=100"},
		{9012, Genders.Male, Filters.CurrentYear,
			"https://www.strava.com/segments/9012?date_range=this_year&filter=current_year&gender=F&per_page=100"},
		{3456, Genders.Female, Filters.CurrentYear,
			"https://www.strava.com/segments/3456?date_range=this_year&filter=current_year&gender=F&per_page=100"},
	}
	for _, tt := range tests {
		actual := getLeaderboardUrl(tt.segmentId, tt.gender, tt.filter)
		if actual != tt.expected {
			t.Errorf("getLeaderboardUrl(%d, %s, %s): got: %s, want: %s",
				tt.segmentId, tt.gender, tt.filter, actual, tt.expected)
		}
	}
}

func TestGetLeaderboardAndSegment(t *testing.T) {
	expectedSegment := &Segment{
		ID:                 2198806,
		Name:               "PCSD",
		Location:           "Dixon, CA",
		Distance:           16.11,
		AverageGrade:       0.00,
		ElevationLow:       83,
		ElevationHigh:      96,
		TotalElevationGain: 13,
	}
	tests := []struct {
		files                                                          []string
		segmentId                                                      int64
		gender                                                         Gender
		filter                                                         Filter
		expectedRequestCount, expectedLenEntries, expectedEntriesCount int64
	}{
		{{"segment-male-overall.1.html", "segment-male-overall.2.html", "segment-male-overall.3.html", "segment-male-overall.4.html", "segment-male-overall.5.html"},
			2198806, Genders.Male, Filters.Overall, 5, 474, 474},
		{{"segment-female-overall.1.html", "segment-female-overall.2.html"}, 2198806, Genders.Female, Filters.Overall, 2, 120, 120},
		{{"segment-male-yearly.1.html"}, 2198806, Genders.Male, Filters.CurrentYear, 1, 21, 21},
		{{"segment-female-yearly.1.html"}, 2198806, Genders.Female, Filters.CurrentYear, 1, 4, 4},
	}
	for _, tt := range tests {
		c := newStubClient(t, files)
		leaderboard, segment, err := client.GetLeaderboardAndSegment(tt.segmentId, tt.gender, tt.filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaderboard.Entries) != tt.expectedLenEntries ||
			leaderboard.EntriesCount != tt.expectedEntriesCount ||
			segment != expectedSegment ||
			c.RequestCount != tt.expectedRequestCount {
			t.Errorf("GetLeaderboardAndSegment(%d, %s, %s): got: ((%d, %d), %v, %d), want: ((%d, %d), %v, %d)",
				tt.segmentId, tt.gender, tt.filter, len(leaderboard.Entries), leaderboard.EntriesCount, segment, c.RequestCount,
				tt.expectedLenEntries, tt.expectedEntriesCount, expectedSegment, tt.ex.expectedRequestCount)
		}
	}
}

//func TestGetLeaderboard(t *testing.T) { }
//func TestGetLeaderboardPageAndSegment(t *testing.T) { }
//func TestGetLeaderboardPage(t *testing.T) { }

func newStubClient(t *testing.T, files []string) *Client {
	var contents []string
	for file := range files {
		content, err := ioutil.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, content)
	}
	return NewStubClient(contents)
}
