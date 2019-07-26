package scheduler

import (
	"github.com/alexkarlov/15x4bot/bot"
	"github.com/alexkarlov/15x4bot/store"
	"github.com/alexkarlov/simplelog"
)

const (
	TEMPLATE_LECTION_DESCRIPTION_REMINDER = `Привіт, %username%!
	По можливості - напиши, будь ласка, опис до своєї лекції. В головному меню є пункт "Додати опис до лекції". Якщо будуть питання - звертайся до @alex_karlov
	Дякую велетенське!
	`
)

// RemindLector sends message to the speaker about description of his lecture
func RemindLector(t *store.Task, b *bot.Bot) {
	log.Info("got new reminder lector:", t)
	err := t.TakeTask()
	if err != nil {
		log.Errorf("failed to take task %d error:%s", t.ID, err)
		return
	}
	l, err := t.LoadLection()
	if err != nil {
		log.Errorf("failed to load lection of task %d error:%s", t.ID, err)
		return
	}
	if l.Description != "" {
		if err = store.FinishTask(t.ID); err != nil {
			log.Errorf("failed to finish task %d error:%s", t.ID, err)
		}
		return
	}
	c, err := l.Lector.TGChat()
	if err != nil {
		log.Error("error while getting tg chat of the lector", err)
		return
	}
	b.SendText(c.TGChatID, TEMPLATE_LECTION_DESCRIPTION_REMINDER)
	store.PostponeTask(t.ID, store.POSTPONE_PERIOD_ONE_DAY)
	log.Info(t)
}
