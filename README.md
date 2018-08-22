# stravax

![version](http://img.shields.io/badge/version-0.1.0-brightgreen.svg)&nbsp;
[![Build Status](http://img.shields.io/travis/scheibo/stravax.svg)](https://travis-ci.org/scheibo/stravax)

stravax extends the Strava API to allow for retrieving complete Strava
leaderboard information for a logged in user.

The generated GoDoc can be viewed at
[godoc.org/github.com/scheibo/stravax](https://godoc.org/github.com/scheibo/stravax).

## Usage

    client, err := stravax.NewClient(email, password)

    leaderboard, err :=
      client.GetLeaderboardPage(
        segmentId, stravax.Genders.Male, stravax.Filters.CurrentYear, 1)

    for _, e := range leaderboard.Entries {
      fmt.Printf("%d) %s: %v (%s)\n",
        e.Rank,
        e.Athlete.Name,
        time.Duration(e.ElapsedTime)*time.Second,
        e.StartDate)
    }
