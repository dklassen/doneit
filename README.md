# Doneit 

Doneit is the package for running tasks against gorm.Models at predetermined intervals. All
tasks are backed by the database to track metadata regarding task execution times
and errors.

Here's an example of scheduling a question to be asked every Wednesday:
```golang
type Question struct {
  gorm.Model
}

func AskQuestion(questionManager *QuestionManager, question *Question) error {
  // Execute some code on the question with access to the db should
  // you want to modify and save the record
}

question := &Question{}
// question must be a record in the db before running if we are going to support
// logging to the db
db.Save(question)

scheduler := NewScheduler(db)
scheduler.For(question).
  Do(AskQuestion, questionStore, question).
  Schedule(1, "days", "Wednesday", "")
```
