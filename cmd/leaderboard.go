// leaderboard provides a CLI for retrieving the full details of a page of a Strava leaderboard.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scheibo/stravax"
)

func main() {
	var email, password, token string
	var segmentId int64

	flag.StringVar(&email, "email", "", "Email")
	flag.StringVar(&password, "password", "", "Password")
	flag.StringVar(&token, "token", "", "Access Token")
	flag.Int64Var(&segmentId, "id", -1, "Segment Id")

	flag.Parse()

	if email == "" {
		exit(fmt.Errorf("Please provide an email"))
	}
	if password == "" {
		exit(fmt.Errorf("Please provide a password"))
	}
	if token == "" {
		exit(fmt.Errorf("Please provide an access token"))
	}
	if segmentId < 0 {
		exit(fmt.Errorf("Please provide a segment"))
	}

	client, err := stravax.NewClient(email, password, token)
	if err != nil {
		exit(err)
	}

	segment, err := client.GetSegment(segmentId)
	if err != nil {
		exit(err)
	}

	leaderboard, err :=
		client.GetLeaderboardPage(segmentId, stravax.Genders.Male, stravax.Filters.CurrentYear, 1)
	if err != nil {
		exit(err)
	}

	fmt.Printf("%s (%d): %.2f km @ %.2f%%\n",
		segment.Name, segment.ID, segment.Distance, segment.AverageGrade)
	for _, e := range leaderboard.Entries {
		fmt.Printf("%d) %s: %v (%s)\n",
			e.Rank,
			e.Athlete.Name,
			fmtDuration(time.Duration(e.ElapsedTime)*time.Second),
			e.StartDate)
	}
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	flag.PrintDefaults()
	os.Exit(1)
}
