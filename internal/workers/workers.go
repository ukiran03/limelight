package workers

import (
	"context"

	"github.com/riverqueue/river"
	"ukiran.com/limelight/internal/mailer"
)

type OnBoardEmailArgs struct {
	Email             string
	EmailTemplateFile string
	EmailData         interface{}
}

// Kind uniquely identifies this type of job in the database
func (OnBoardEmailArgs) Kind() string {
	return "send-onboarding-email"
}

type OnBoardEmailWorker struct {
	river.WorkerDefaults[OnBoardEmailArgs]
	M *mailer.Mailer
}

func (w *OnBoardEmailWorker) Work(
	ctx context.Context, job *river.Job[OnBoardEmailArgs],
) error {
	args := job.Args
	err := w.M.Send(ctx, args.Email, args.EmailTemplateFile, args.EmailData)
	if err != nil {
		return err
	}
	return nil
}
