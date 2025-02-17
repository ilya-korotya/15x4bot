package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/alexkarlov/15x4bot/store"
)

var (
	ErrWrongPlace     = errors.New("wrong place id: failed to conver from string to int")
	ErrWrongRehearsal = errors.New("wrong rehearsal id: failed to convert from string to int")
)

const (
	ChannelUsernameInternal = "@odessa15x4_org"
	ChatInternalID          = "-1001163442626"
	RemindHourStart         = 10
	RemindHourEnd           = 21

	TEMPLATE_NEXT_REHEARSAL                  = "Де: %s, %s\nКоли: %s\nМапа:%s"
	TEMPLATE_ADD_REHEARSAL_WHEN              = "Коли? Дата та час в форматі 2018-12-31 19:00:00"
	TEMPLATE_ADD_REHEARSAL_ERROR_DATE        = "Невірний формат дати та часу. Наприклад, якщо репетиція буде 20-ого грудня о 19:00 то треба ввести: 2018-12-20 19:00:00. Спробуй ще!"
	TEMPLATE_ADDREHEARSAL_SUCCESS_MSG        = "Репетиція створена"
	TEMPLATE_NEXT_REHEARSAL_UNDEFINED        = "Невідомо коли, спитай пізніше"
	TEMPLATE_REHEARSAL_BUTTON                = "%d.Коли: %s, місце: %s"
	TEMPLATE_CHOSE_REHEARSAL                 = "Оберіть репетицію"
	TEMPLATE_REHEARSAL_ERROR_WRONG_ID        = "Невірно вибрана репетиція"
	TEMPLATE_DELETE_REHEARSAL_COMPLETE       = "Репетиція успішно видалена"
	TEMPLATE_ADDREHEARSAL_ERROR_REMINDER_MSG = "Репетиція створена, але якась фігня скоїлась при створенні нагадувань. Звернись пліз до @alex_karlov"
)

type addRehearsal struct {
	step  int
	when  time.Time
	where int
	u     *store.User
}

func (c *addRehearsal) IsAllow(u *store.User) bool {
	c.u = u
	return u.Role == store.USER_ROLE_ADMIN
}

func (c *addRehearsal) NextStep(answer string) (*ReplyMarkup, error) {
	replyMarkup := &ReplyMarkup{
		Buttons: MainMarkup,
	}
	var err error
	switch c.step {
	case 0:
		replyMarkup.Text = TEMPLATE_ADD_REHEARSAL_WHEN
	case 1:
		t, err := time.Parse("2006-01-02 15:04:05", answer)
		if err != nil {
			replyMarkup.Text = TEMPLATE_ADD_REHEARSAL_ERROR_DATE
			return replyMarkup, nil
		}
		c.when = t
		places, err := store.Places(store.PlaceTypes{store.PLACE_TYPE_FOR_REHEARSAL, store.PLACE_TYPE_FOR_ALL})
		replyMarkup.Buttons = nil
		for _, p := range places {
			b := fmt.Sprintf(TEMPLATE_PLACES_LIST_BUTTONS, p.ID, p.Name)
			replyMarkup.Buttons = append(replyMarkup.Buttons, b)
		}
		replyMarkup.Text = TEMPLATE_CHOSE_PLACE
	case 2:
		regexpLectionID := regexp.MustCompile(`^(\d+)?\.`)
		matches := regexpLectionID.FindStringSubmatch(answer)
		if len(matches) < 2 {
			return nil, ErrWrongPlace
		}
		c.where, err = strconv.Atoi(matches[1])
		if err != nil {
			return nil, ErrWrongPlace
		}
		ok, err := store.DoesPlaceExist(c.where)
		if err != nil {
			return nil, err
		}
		if !ok {
			replyMarkup.Text = TEMPLATE_WRONG_PLACE_ID
			return replyMarkup, nil
		}
		id, err := store.AddRehearsal(c.when, c.where)
		// create a task for sending post in the internal channel
		if err != nil {
			return nil, err
		}
		err = addRehearsalReminder(id)
		if err != nil {
			replyMarkup.Text = TEMPLATE_ADDREHEARSAL_ERROR_REMINDER_MSG
			break
		}

		replyMarkup.Text = TEMPLATE_ADDREHEARSAL_SUCCESS_MSG
		replyMarkup.Buttons = StandardMarkup(c.u.Role)
	}
	c.step++
	return replyMarkup, nil
}

func addRehearsalReminder(id int) error {
	// create chat reminder
	r := &store.RemindRehearsalChannel{
		RehearsalID:     id,
		ChannelUsername: ChatInternalID,
	}
	details, err := json.Marshal(r)
	if err != nil {
		return err
	}
	execTime := asSoonAsPossible()
	err = store.AddTask(store.TASK_TYPE_REMINDER_TG_CHANNEL, execTime, string(details))
	if err != nil {
		return err
	}
	// create channel reminder
	r.ChannelUsername = ChannelUsernameInternal
	details, err = json.Marshal(r)
	if err != nil {
		return err
	}
	return store.AddTask(store.TASK_TYPE_REMINDER_TG_CHANNEL, execTime, string(details))
}

func (c *addRehearsal) IsEnd() bool {
	return c.step == 3
}

type nextRep struct {
	u *store.User
}

func (c *nextRep) IsEnd() bool {
	return true
}

func (c *nextRep) IsAllow(u *store.User) bool {
	c.u = u
	return true
}

func (c *nextRep) NextStep(answer string) (*ReplyMarkup, error) {
	replyMarkup := &ReplyMarkup{
		Buttons: StandardMarkup(c.u.Role),
	}
	r, err := store.NextRehearsal()
	if err != nil {
		if err == store.ErrUndefinedNextRehearsal {
			replyMarkup.Text = TEMPLATE_NEXT_REHEARSAL_UNDEFINED
			return replyMarkup, nil
		}
		return nil, err
	}
	replyMarkup.Text = fmt.Sprintf(TEMPLATE_NEXT_REHEARSAL, r.PlaceName, r.Address, r.Time.Format("2006-01-02 15:04:05"), r.MapUrl)
	return replyMarkup, nil
}

type deleteRehearsal struct {
	step        int
	rehearsalID int
	u           *store.User
}

func (c *deleteRehearsal) IsEnd() bool {
	return c.step == 2
}

func (c *deleteRehearsal) IsAllow(u *store.User) bool {
	c.u = u
	return u.Role == store.USER_ROLE_ADMIN
}

func (c *deleteRehearsal) NextStep(answer string) (*ReplyMarkup, error) {
	replyMarkup := &ReplyMarkup{
		Buttons: MainMarkup,
	}
	switch c.step {
	case 0:
		rehearsals, err := store.Rehearsals()
		if err != nil {
			return nil, err
		}
		for _, r := range rehearsals {
			lText := fmt.Sprintf(TEMPLATE_REHEARSAL_BUTTON, r.ID, r.Time, r.PlaceName)
			replyMarkup.Buttons = append(replyMarkup.Buttons, lText)
		}
		replyMarkup.Text = TEMPLATE_CHOSE_REHEARSAL
	case 1:
		regexpRehearsalID := regexp.MustCompile(`^(\d+)?\.`)
		matches := regexpRehearsalID.FindStringSubmatch(answer)
		if len(matches) < 2 {
			replyMarkup.Text = TEMPLATE_REHEARSAL_ERROR_WRONG_ID
			return replyMarkup, nil
		}
		rID, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, ErrWrongRehearsal
		}
		err = store.DeleteRehearsal(rID)
		if err != nil {
			return nil, err
		}
		replyMarkup.Buttons = StandardMarkup(c.u.Role)
		replyMarkup.Text = TEMPLATE_DELETE_REHEARSAL_COMPLETE
	}
	c.step++
	return replyMarkup, nil
}

// asSoonAsPossible returns nearest time  according to start/end hour
// i.e. we have start hour = 10, end hour = 21
// current time = 2019-01-01 12:00:00
// asSoonAsPossible returns 2019-01-01 12:00:00
// if current time = 2019-01-01 09:00:00
// asSoonAsPossible returns 2019-01-01 10:00:00
// if current time = 2019-01-01 22:00:00
// asSoonAsPossible returns 2019-01-02 10:00:00
func asSoonAsPossible() time.Time {
	loc, _ := time.LoadLocation(MsgLocation)
	curr := time.Now().In(loc)
	y, m, d := curr.Date()
	currH := curr.Hour()
	if currH < RemindHourStart {
		return time.Date(y, m, d, RemindHourStart, 0, 0, 0, loc)
	} else if currH > RemindHourEnd {
		// next day at as early as possible
		return time.Date(y, m, d, RemindHourStart, 0, 0, 0, loc).AddDate(0, 0, 1)
	}
	return curr
}
