package doneit

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/Sirupsen/logrus"
)

// Unit specifices the time unit (Minutes = 0, ...)
type TimeUnit int

const (
	Seconds TimeUnit = iota
	Minutes
	Hours
	Days
	Weeks
)

var (
	loc   = time.Local
	units = [...]string{
		"Seconds",
		"Minutes",
		"Hours",
		"Days",
		"Weeks",
	}
)

// TaskQueue so we can implement sortable task queue
type TaskQueue []Tasker

func (q TaskQueue) Len() int {
	return len(q)
}

func (q TaskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q TaskQueue) Less(i, j int) bool {
	return q[j].NextRun().After(q[i].NextRun())
}

// StringToUnit converts a string to a TimeUnit
func StringToUnit(unit string) TimeUnit {
	var u TimeUnit
	switch unit {
	case "Seconds":
		u = Seconds
	case "Minutes":
		u = Minutes
	case "Hours":
		u = Hours
	case "Days":
		u = Days
	case "Weeks":
		u = Weeks
	default:
		logrus.Panic(fmt.Sprintf("Invalid time unit option %s", unit))
	}
	return u
}

func (u TimeUnit) String() string { return units[u] }

// Weekday returns the time.Weekday value for a passed in string representation
// of the weekday
func Weekday(day string) time.Weekday {
	var weekday time.Weekday
	switch day {
	case "Sunday":
		weekday = time.Sunday
	case "Monday":
		weekday = time.Monday
	case "Tuesday":
		weekday = time.Tuesday
	case "Wednesday":
		weekday = time.Wednesday
	case "Thursday":
		weekday = time.Thursday
	case "Friday":
		weekday = time.Friday
	case "Saturday":
		weekday = time.Saturday
	default:
		logrus.Panic(fmt.Sprintf("Invalid day option %s", day))
	}
	return weekday
}

// Scheduler holds tasks which are to be triggered and run on a scheduled
type Scheduler struct {
	Tasks    []Tasker
	TaskType reflect.Type
}

// Tasker is any object that we can schedule to execute a func against
// at a regular interval.
type Tasker interface {
	Schedule(int, string, string, string) *Task
	ScheduleNextRun()
	ShouldRun() bool
	SetLastRun()
	NextRun() time.Time
	LastRun() time.Time
	TimeKey() string
	Execute() error
	Do(interface{}, ...interface{}) *Task
}

// Task describes a scheduled job that will be run at a regular interval
type Task struct {
	// The number of units between jobs runs
	Interval int `yaml:"interval"`

	// When the job will be run next
	NextRunField time.Time

	// When the job was last run
	LastRunField time.Time

	// seconds, minutes, hours, days, weeks
	Unit TimeUnit `yaml:"unit"`

	// At is a string representation of time of day a task should run 10:00PM
	At string `yaml:"at"`

	// Day of week the job is started on
	StartDay time.Weekday `yaml:"startday"`

	// Cached Combination of interval and unit (i.e 2 weeks)
	period time.Duration

	// Task is an interface holding the func we are going to execute each period.
	TaskFunc interface{} `sql:"-"`

	// TaskParams is an interface holding the parameters passed into the
	// Task func.
	TaskParams []interface{} `sql:"-"`
}

func NewTask() *Task {
	return &Task{
		Interval:     0,
		NextRunField: time.Unix(0, 0),
		LastRunField: time.Unix(0, 0),
		Unit:         Hours,
		StartDay:     time.Monday,
		period:       0,
	}
}

// NewTask creates a Task with the default parameters
func (s *Scheduler) NewTask() *Task {
	task := NewTask()
	s.Tasks = append(s.Tasks, task)
	return task
}

// CurrentTime is func to return the current time time.Now()
var CurrentTime = func() time.Time {
	return time.Now()
}

// Key returns a representation of a task that can be used for indexing a
// map of Tasks. Key's are not unique across tasks.
func (t *Task) TimeKey() string {
	min := 0
	hour := 0
	if t.At != "" {
		hour, min, _ = ParseAt(t.At)
	}
	return fmt.Sprintf("%d%d%d%02d%02d", t.StartDay, t.Unit, t.Interval, hour, min)
}

// ShouldRun returns a bool based on if the Task run time is after
// the CurrentTime
func (t *Task) ShouldRun() bool {
	return CurrentTime().After(t.NextRun())
}

// NextRun returns the field value for when the Task should next run.
func (t *Task) NextRun() time.Time {
	return t.NextRunField
}

// LastRun returns the field value for when the Task was last run.
func (t *Task) LastRun() time.Time {
	return t.LastRunField
}

// PreviousWeekdayFrom calculates the previous relative specified Weekday to the
// time passed in time and number of previous weeks to look back
func PreviousWeekdayFrom(when time.Time, start time.Weekday, weeks int) time.Time {
	dayDiff := when.Weekday() - start
	if dayDiff < 0 {
		dayDiff = 7 + dayDiff
	}

	modified := when.Add(-time.Duration(dayDiff) * 24 * time.Hour)

	if weeks > 1 {
		return modified.AddDate(0, 0, -7*weeks)
	}
	return modified
}

// NextWeekdayFrom returns  the mext specified weekday relative to the time
// passed in and number of weeks to look forward
func NextWeekdayFrom(when time.Time, start time.Weekday, weeks int) time.Time {
	dayDiff := when.Weekday() - start
	if dayDiff < 0 {
		dayDiff = 7 + dayDiff
	}

	modified := when.Add(-time.Duration(dayDiff) * 24 * time.Hour)

	return modified.AddDate(0, 0, 7*weeks)
}

// SetLastRun calculates when a task should have last run.
func (t *Task) SetLastRun() {
	now := CurrentTime()
	lastTick := now

	if t.At != "" {
		hour, min, err := ParseAt(t.At)
		if err != nil {
			logrus.Panic(err)
		}
		lastTick = time.Date(now.Year(), now.Month(), now.Day(), int(hour), int(min), 0, 0, loc)
	}

	switch t.Unit {
	case Days:
		if now.After(lastTick) {
			t.LastRunField = lastTick.Add(-24 * time.Hour)
		} else {
			t.LastRunField = lastTick
		}
	case Weeks:
		if now.After(lastTick) || t.LastRunField == time.Unix(0, 0) {
			t.LastRunField = PreviousWeekdayFrom(now, t.StartDay, 1)
		} else {
			t.LastRunField = lastTick.Add(-7 * 24 * time.Hour)
		}
	default:
		t.LastRunField = lastTick
	}
}

// ParseAt takes a string representing a clock time and converts to separate
// hours and min
func ParseAt(at string) (int, int, error) {
	form := "3:04PM"
	atTime, err := time.Parse(form, at)
	if err != nil {
		return 0, 0, err
	}
	return atTime.Hour(), atTime.Minute(), nil
}

// Schedule sets the timing elements of Task. We make sure we can parse the At string
// before assignment to catch errors early
func (t *Task) Schedule(interval int, unit string, startDay string, at string) *Task {
	t.Interval = interval
	t.Unit = StringToUnit(unit)

	if startDay != "" {
		t.StartDay = Weekday(startDay)
	}

	if at != "" {
		_, _, err := ParseAt(at)
		if err != nil {
			logrus.Panic(err)
		}
		t.At = at
	}
	return t
}

// ScheduleNextRun looks at the LastRun and period to determine when the
// task should run next
func (t *Task) ScheduleNextRun() {
	if t.LastRunField == time.Unix(0, 0) {
		t.SetLastRun()
	}

	if t.period != 0 {
		t.NextRunField = t.LastRunField.Add(t.period)
		return
	}

	var period time.Duration
	switch t.Unit {
	case Seconds:
		period = time.Duration(t.Interval)
	case Minutes:
		period = time.Duration(t.Interval) * time.Minute
	case Hours:
		period = time.Duration(t.Interval) * time.Hour
	case Days:
		period = time.Duration(t.Interval*24) * time.Hour
	case Weeks:
		period = time.Duration(t.Interval*24*7) * time.Hour
	}

	t.period = period
	t.NextRunField = t.LastRunField.Add(t.period)
}

// Do is a generic task function that is executed at the scheduled interval
// We will only be passing in the target gorm.Model in most cases unless
// it is a generic task which requires known parameters at the start
func (t *Task) Do(f interface{}, fparams ...interface{}) *Task {

	typ := reflect.TypeOf(f)
	if typ.Kind() != reflect.Func {
		logrus.Panic("Task must be a function!")
	}

	t.TaskFunc = f
	t.TaskParams = fparams

	t.ScheduleNextRun()

	return t
}

// Execute is the basic execution of a task with params supplied at task CreateQuestion
// we reflect the function and params
func (t *Task) Execute() error {
	f := reflect.ValueOf(t.TaskFunc)
	params := t.TaskParams

	if len(params) != f.Type().NumIn() {
		return fmt.Errorf("Expected %d params got: %d", f.Type().NumIn(), len(params))
	}

	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}

	f.Call(in)
	t.LastRunField = CurrentTime()
	t.ScheduleNextRun()
	return nil
}

// NewScheduler returns an initialized scheduler struct
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// PendingTasks returns tasks that have NextRun times after CurrentTime
func (s *Scheduler) PendingTasks() []Tasker {
	pendingTasks := []Tasker{}
	for _, task := range s.Tasks {
		if task.ShouldRun() {
			pendingTasks = append(pendingTasks, task)
		}
	}
	sort.Sort(TaskQueue(pendingTasks))
	return pendingTasks
}

// RunPendingTasks runs all jobs that are scheduled to run.
// Tasks are execute serially so long running tasks will back up
func (s *Scheduler) RunPendingTasks() {
	tasksToExecute := s.PendingTasks()
	for _, task := range tasksToExecute {
		errMsg := ""
		if err := task.Execute(); err != nil {
			errMsg = err.Error()
		}

		timeFinished := CurrentTime()

		logrus.WithFields(logrus.Fields{
			"task_Last_run": task.LastRun(),
			"task_next_run": task.NextRun(),
			"time_finished": timeFinished,
			"time_taken":    timeFinished.Sub(task.LastRun()).Seconds(),
			"error":         errMsg,
		}).Info("Finished Task")
	}
}

// Start the schedule and scan for runable jobs every minute
// Note:: Granularity is every minute jobs scheduled for lower time
// will be missed for now untill its changed
func (s *Scheduler) Start() chan bool {
	closed := make(chan bool)
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				s.RunPendingTasks()
			case <-closed:
				ticker.Stop()
				return
			}
		}
	}()

	return closed
}
