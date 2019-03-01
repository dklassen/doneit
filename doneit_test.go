package doneit

import (
	"os"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/kylelemons/godebug/pretty"
	_ "github.com/lib/pq"
)

var db *gorm.DB

func init() {
	var err error

	os.Setenv("GOENV", "test")
	debug := false
	debugEnv := os.Getenv("DEBUG")
	if debugEnv == "true" {
		debug = true
	}

	databaseURL := "postgres://postgres:@127.0.0.1/osmosis_test?sslmode=disable"

	db, err = gorm.Open("postgres", databaseURL)
	if err != nil {
		logrus.Panic(err)
	}
	db.LogMode(debug)
}

type TestModel struct {
	gorm.Model
}

func TestTaskTimeKey(t *testing.T) {
	testCases := []struct {
		Input  Task
		Output string
	}{
		{
			// At value is included
			Input:  Task{Interval: 1, Unit: Seconds, StartDay: time.Monday, At: "10:20AM"},
			Output: "1011020",
		},
		{
			// No At value is considered 0 hour and min
			Input:  Task{Interval: 1, Unit: Seconds, StartDay: time.Monday},
			Output: "1010000",
		},
	}

	for _, testCase := range testCases {
		result := testCase.Input.TimeKey()
		if result != testCase.Output {
			t.Fatalf("Expected key %s to equal %s", result, testCase.Output)
		}
	}
}

func TestDoTask(t *testing.T) {
	var testresult bool = false
	testDo := func() error {
		testresult = true
		return nil
	}

	scheduler := NewScheduler()
	task := scheduler.NewTask().
		Do(testDo).
		Schedule(1, "Days", "Wednesday", "")

	if err := task.Execute(); err != nil {
		t.Error(err)
	}

	if !testresult {
		t.Error("Expected result to be true")
	}
}

func TestDoWithParams(t *testing.T) {
	testresult := false
	testDo := func(addedParam bool) error {
		testresult = addedParam
		return nil
	}

	scheduler := NewScheduler()
	task := scheduler.NewTask().
		Do(testDo, true).
		Schedule(1, "Days", "Wednesday", "")

	if err := task.Execute(); err != nil {
		t.Error(err)
	}

	if !testresult {
		t.Error("Expected result to be true")
	}
}

func TestParseAt(t *testing.T) {
	var table = []struct {
		At         string
		ExpectHour int
		ExpectMin  int
	}{
		{At: "11:00PM", ExpectHour: 23, ExpectMin: 0},
		{At: "12:00PM", ExpectHour: 12, ExpectMin: 0},
		{At: "12:00AM", ExpectHour: 0, ExpectMin: 0},
	}

	for _, testCase := range table {
		hour, min, _ := ParseAt(testCase.At)
		if hour != testCase.ExpectHour || min != testCase.ExpectMin {
			t.Errorf("Expected %s to convert to %d hour and %d got: %d and %d", testCase.At, testCase.ExpectHour, testCase.ExpectMin, hour, min)
		}
	}
}

func TestCreateAndSchedule(t *testing.T) {

	// All days relative from Thursday Jan 12
	now := time.Date(2017, 1, 12, 22, 33, 38, 111, loc)

	previousFriday := time.Date(2017, 1, 6, 22, 33, 38, 111, loc)
	nextFriday := time.Date(2017, 1, 13, 22, 33, 38, 111, loc)

	previousWednesday := time.Date(2017, 1, 11, 22, 33, 38, 111, loc)
	nextWednesday := time.Date(2017, 1, 18, 22, 33, 38, 111, loc)

	previousSunday := time.Date(2017, 1, 8, 22, 33, 38, 111, loc)
	nextSunday := time.Date(2017, 1, 15, 22, 33, 38, 111, loc)
	twoSundaysFromNow := time.Date(2017, 1, 22, 22, 33, 38, 111, loc)

	CurrentTime = func() time.Time {
		return now
	}

	var testTable = []struct {
		Interval        int
		Unit            string
		Day             string
		At              string
		ExpectedLastRun time.Time
		ExpectedNextRun time.Time
	}{
		{1, "Days", "Sunday", "", now, now.Add(24 * time.Hour)},
		{2, "Days", "Sunday", "", now, now.Add(48 * time.Hour)},
		{1, "Weeks", "Friday", "", previousFriday, nextFriday},
		{1, "Weeks", "Wednesday", "", previousWednesday, nextWednesday},
		{1, "Weeks", "Sunday", "", previousSunday, nextSunday},
		{2, "Weeks", "Sunday", "", previousSunday, twoSundaysFromNow},
	}

	for _, testCase := range testTable {
		scheduler := NewScheduler()
		task := scheduler.NewTask().
			Schedule(testCase.Interval, testCase.Unit, testCase.Day, testCase.At)

		task.ScheduleNextRun()
		if !task.LastRun().Equal(testCase.ExpectedLastRun) {
			t.Errorf("interval: %v, unit: %s, day: %s, at: %s", task.Interval, task.Unit, task.StartDay, task.At)
			t.Errorf("Expected last run %s got: %s", testCase.ExpectedLastRun, task.LastRun())
		}

		if !task.NextRun().Equal(testCase.ExpectedNextRun) {
			t.Errorf("Expected next run %s got: %s", testCase.ExpectedNextRun, task.NextRun())
		}
	}
}

func TestPreviousWeekDayFrom(t *testing.T) {
	var cases = []struct {
		When         time.Time
		Weekday      time.Weekday
		Weeks        int
		ExpectedWhen time.Time
	}{
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Sunday, 1, time.Date(2017, 1, 1, 1, 1, 1, 0, loc)},
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Monday, 1, time.Date(2017, 1, 2, 1, 1, 1, 0, loc)},
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Tuesday, 1, time.Date(2016, 12, 27, 1, 1, 1, 0, loc)},
	}

	for _, tc := range cases {
		result := PreviousWeekdayFrom(tc.When, tc.Weekday, tc.Weeks)
		if !result.Equal(tc.ExpectedWhen) {
			t.Errorf("Expected next weekday: %s, but got: %s", tc.ExpectedWhen, result)
		}
	}
}

func TestNextWeekDayFrom(t *testing.T) {
	var cases = []struct {
		When         time.Time
		Weekday      time.Weekday
		Weeks        int
		ExpectedWhen time.Time
	}{
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Sunday, 1, time.Date(2017, 1, 8, 1, 1, 1, 0, loc)},
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Monday, 1, time.Date(2017, 1, 9, 1, 1, 1, 0, loc)},
		{time.Date(2017, 1, 2, 1, 1, 1, 0, loc), time.Tuesday, 1, time.Date(2017, 1, 3, 1, 1, 1, 0, loc)},
	}

	for _, tc := range cases {
		result := NextWeekdayFrom(tc.When, tc.Weekday, tc.Weeks)
		if !result.Equal(tc.ExpectedWhen) {
			t.Errorf("Expected next weekday: %s, but got: %s", tc.ExpectedWhen, result)
		}
	}
}

func TestPendingTasksAreOrdered(t *testing.T) {
	now := time.Date(2017, 1, 1, 1, 1, 1, 0, loc)
	CurrentTime = func() time.Time {
		return now
	}

	tasks := []Tasker{
		// two days from now
		&Task{
			Interval:     1,
			NextRunField: now.Add(time.Duration(48) * time.Hour),
			LastRunField: now,
			Unit:         Days,
		},
		// one day from now
		&Task{
			Interval:     1,
			NextRunField: now.Add(time.Duration(24) * time.Hour),
			LastRunField: now,
			Unit:         Days,
		},
	}

	scheduler := NewScheduler()
	scheduler.Tasks = tasks

	// Current time is now set to sometime in the future ahead of scheduled tasks
	CurrentTime = func() time.Time {
		return now.Add(time.Duration(72) * time.Hour)
	}

	resultTasks := scheduler.PendingTasks()
	expectedRunTimes := []time.Time{
		now.Add(time.Duration(24) * time.Hour),
		now.Add(time.Duration(48) * time.Hour),
	}

	resultRunTimes := []time.Time{}
	for _, task := range resultTasks {
		resultRunTimes = append(resultRunTimes, task.NextRun())
	}

	if diff := pretty.Compare(resultRunTimes, expectedRunTimes); diff != "" {
		t.Errorf("diff: (-got +want)\n%s", diff)
	}
}

func TestWeekday(t *testing.T) {
	var cases = []struct {
		Day      string
		Expected time.Weekday
	}{
		{"Sunday", time.Sunday},
		{"Monday", time.Monday},
		{"Tuesday", time.Tuesday},
		{"Wednesday", time.Wednesday},
		{"Thursday", time.Thursday},
		{"Friday", time.Friday},
		{"Saturday", time.Saturday},
	}

	for _, test := range cases {
		result := Weekday(test.Day)

		if result != test.Expected {
			t.Errorf("Expected %s to give %d, got: %d", test.Day, test.Expected, result)
		}
	}
}

func TestRunPendingTasks(t *testing.T) {
	now := time.Date(2017, 1, 1, 1, 1, 1, 0, loc)

	CurrentTime = func() time.Time {
		return now
	}

	testCases := []struct {
		input           *Task
		ExpectedNextRun time.Time
	}{
		{
			input: &Task{
				Interval:     1,
				LastRunField: now,
				Unit:         Hours,
				TaskFunc:     func() {},
			},
			ExpectedNextRun: now.Add(time.Duration(1) * time.Hour),
		},
		{
			input: &Task{
				Interval:     15,
				LastRunField: now,
				Unit:         Minutes,
				TaskFunc:     func() {},
			},
			ExpectedNextRun: now.Add(time.Duration(15) * time.Minute),
		},
	}

	for _, testCase := range testCases {
		scheduler := NewScheduler()
		scheduler.Tasks = append(scheduler.Tasks, testCase.input)
		scheduler.RunPendingTasks()

		task := scheduler.Tasks[0]
		if task.NextRun() != testCase.ExpectedNextRun {
			t.Fatalf("Expected last run %s got: %s", testCase.ExpectedNextRun, task.NextRun())
		}
	}
}
