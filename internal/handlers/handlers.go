package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/zahnah/study-app/internal/config"
	"github.com/zahnah/study-app/internal/forms"
	"github.com/zahnah/study-app/internal/helpers"
	"github.com/zahnah/study-app/internal/models"
	"github.com/zahnah/study-app/internal/render"
	"github.com/zahnah/study-app/repository"
	"github.com/zahnah/study-app/repository/dbrepo"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var Repo *Repository

type Repository struct {
	App *config.AppConfig
	DB  repository.DatabaseRepo
}

func NewRepo(app *config.AppConfig, db *sql.DB) *Repository {
	return &Repository{
		App: app,
		DB:  dbrepo.NewPostgresRepo(db, app),
	}
}

func NewTestRepo(app *config.AppConfig) *Repository {
	return &Repository{
		App: app,
		DB:  dbrepo.NewTestRepo(app),
	}
}

func NewHandlers(r *Repository) {
	Repo = r
}

// Home is the home page handler
func (m *Repository) Home(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "home.page.gohtml", &models.TemplateData{})
}

// About is the about page handler
func (m *Repository) About(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "about.page.gohtml", &models.TemplateData{})
}

func (m *Repository) Generals(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "generals.page.gohtml", &models.TemplateData{
		IntMap: map[string]int{
			"room_id": 1,
		},
	})
}

func (m *Repository) Contact(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "contact.page.gohtml", &models.TemplateData{})
}

func (m *Repository) Majors(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "majors.page.gohtml", &models.TemplateData{
		IntMap: map[string]int{
			"room_id": 2,
		},
	})
}

func (m *Repository) SearchAvailability(writer http.ResponseWriter, r *http.Request) {
	_ = render.Template(writer, *r, "search-availability.page.gohtml", &models.TemplateData{})
}

func (m *Repository) PostAvailability(writer http.ResponseWriter, r *http.Request) {

	sd := r.Form.Get("start")
	layout := "2006-01-02"
	startDate, err := time.Parse(layout, sd)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "cannot parse start date!")
		http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
		return
	}

	ed := r.Form.Get("end")
	endDate, err := time.Parse(layout, ed)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "cannot parse end date!")
		http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
		return
	}

	rooms, err := m.DB.SearchAvailabilityForAllRooms(startDate, endDate)
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "cannot search for availability!")
		http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if len(rooms) == 0 {
		m.App.Session.Put(r.Context(), "error", "No availability")
		http.Redirect(writer, r, "/search-availability", http.StatusSeeOther)
		return
	} else {
		for _, i := range rooms {
			m.App.InfoLog.Println("ROOM", i.ID, i.RoomName)
		}
	}

	data := make(map[string]interface{})
	data["rooms"] = rooms

	res := models.Reservation{
		StartDate: startDate,
		EndDate:   endDate,
	}
	m.App.Session.Put(r.Context(), "reservation", res)

	_ = render.Template(writer, *r, "choose-room.page.gohtml", &models.TemplateData{
		Data: data,
	})
}

type jsonResponse struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	RoomID    int    `json:"room_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (m *Repository) PostAvailabilityJSON(writer http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		resp := jsonResponse{
			OK:      false,
			Message: "Internal server error",
		}

		out, _ := json.MarshalIndent(resp, "", "     ")
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write(out)
		return
	}

	sd := r.Form.Get("start")
	ed := r.Form.Get("end")
	roomID, _ := strconv.Atoi(r.Form.Get("room_id"))

	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)

	available, err := m.DB.SearchAvailabilityByRoomID(startDate, endDate, roomID)

	if err != nil {
		resp := jsonResponse{
			OK:      false,
			Message: "Couldn't find data",
		}

		out, _ := json.MarshalIndent(resp, "", "     ")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(out)
		return
	}

	resp := jsonResponse{
		OK:        available,
		Message:   "Available! Do you want to make a reservation?",
		RoomID:    roomID,
		StartDate: sd,
		EndDate:   ed,
	}
	if !available {
		resp.Message = "Not Available"
	}

	out, _ := json.MarshalIndent(resp, "", "     ")

	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(out)
}

func (m *Repository) MakeReservation(writer http.ResponseWriter, request *http.Request) {

	res, ok := m.App.Session.Get(request.Context(), "reservation").(models.Reservation)
	if !ok {
		m.App.Session.Put(request.Context(), "error", "cannot get reservation from session")
		http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
		return
	}

	sd := res.StartDate.Format("2006-01-02")
	ed := res.EndDate.Format("2006-01-02")

	room, err := m.DB.GetRoomById(res.RoomID)
	if err != nil {
		m.App.Session.Put(request.Context(), "error", "cannot find room!")
		http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
		return
	}
	res.Room = room

	m.App.Session.Put(request.Context(), "reservation", res)

	_ = render.Template(writer, *request, "make-reservation.page.gohtml", &models.TemplateData{
		Form: forms.New(nil),
		Data: map[string]interface{}{
			"reservation": res,
		},
		StringMap: map[string]string{
			"StartDate": sd,
			"EndDate":   ed,
		},
	})
}

func (m *Repository) PostMakeReservation(writer http.ResponseWriter, r *http.Request) {

	reservation, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		m.App.Session.Put(r.Context(), "error", "cannot find reservation in session")
		http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
		return
	}

	err := r.ParseForm()
	if err != nil {
		m.App.Session.Put(r.Context(), "error", "cannot parse form!")
		http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
		return
	}

	reservation.FirstName = r.Form.Get("first_name")
	reservation.LastName = r.Form.Get("last_name")
	reservation.Email = r.Form.Get("email")
	reservation.Phone = r.Form.Get("phone")

	form := forms.New(r.PostForm)
	data := make(map[string]interface{})
	data["reservation"] = reservation

	// form.Has("first_name", r)
	form.Required("first_name", "last_name", "email")
	form.MinLength("first_name", 3)
	form.IsEmail("email")

	if !form.Valid() {
		_ = render.Template(writer, *r, "make-reservation.page.gohtml", &models.TemplateData{
			Form: form,
			Data: data,
		})
		return
	} else {
		newID, err := m.DB.InsertReservation(reservation)
		if err != nil {
			m.App.Session.Put(r.Context(), "error", "cannot insert a reservation!")
			http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
			return
		}
		reservation.ID = newID

		restriction := models.RoomRestriction{
			RestrictionID: 1,
			ReservationID: newID,
			RoomID:        reservation.RoomID,
			StartDate:     reservation.StartDate,
			EndDate:       reservation.EndDate,
		}

		_, err = m.DB.InsertRoomRestriction(restriction)
		if err != nil {
			m.App.Session.Put(r.Context(), "error", "cannot insert a reservation's restriction!")
			http.Redirect(writer, r, "/", http.StatusTemporaryRedirect)
			return
		}

		// sending email notification
		htmlMessage := fmt.Sprintf(`<b>Reservation confirmation</b><br>
Dear %s:, <br>
This is confirm your reservation from %s to %s
`, reservation.FirstName, reservation.StartDate.Format("2006-01-02"), reservation.EndDate.Format("2006-01-02"))
		msg := models.MailData{
			To:       reservation.Email,
			From:     "me@local.local",
			Subject:  "Reservation Confirmation",
			Content:  htmlMessage,
			Template: "basic",
		}
		m.App.MailChan <- msg

		htmlMessage = fmt.Sprintf(`<b>Reservation confirmation</b><br>
A reservation has been made for %s from %s to %s
`, reservation.Room.RoomName, reservation.StartDate.Format("2006-01-02"), reservation.EndDate.Format("2006-01-02"))
		msg = models.MailData{
			To:      "owner@email.local",
			From:    "me@local.local",
			Subject: "Reservation Confirmation",
			Content: htmlMessage,
		}
		m.App.MailChan <- msg

		m.App.Session.Put(r.Context(), "flash", "Data stored successfully")
		m.App.Session.Put(r.Context(), "reservation", reservation)
		http.Redirect(writer, r, "/reservation-summary", http.StatusSeeOther)
	}

}

func (m *Repository) ReservationSummary(writer http.ResponseWriter, r *http.Request) {
	reservation, ok := m.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		err := errors.New("can't get item from session")
		helpers.ServerError(writer, err)
		return
	} else {
		m.App.Session.Remove(r.Context(), "reservation")

		sd := reservation.StartDate.Format("2006-01-02")
		ed := reservation.EndDate.Format("2006-01-02")

		_ = render.Template(writer, *r, "reservation-summary.page.gohtml", &models.TemplateData{
			Form: forms.New(nil),
			Data: map[string]interface{}{
				"reservation": reservation,
			},
			StringMap: map[string]string{
				"StartDate": sd,
				"EndDate":   ed,
			},
		})
	}
}

func (m *Repository) ChooseRoom(writer http.ResponseWriter, request *http.Request) {
	roomID, err := strconv.Atoi(chi.URLParam(request, "id"))
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	res, ok := m.App.Session.Get(request.Context(), "reservation").(models.Reservation)
	if !ok {
		helpers.ServerError(writer, err)
		return
	}

	res.RoomID = roomID
	m.App.Session.Put(request.Context(), "reservation", res)

	http.Redirect(writer, request, "/make-reservation", http.StatusSeeOther)
}

func (m *Repository) BookRoom(writer http.ResponseWriter, request *http.Request) {
	roomID, _ := strconv.Atoi(request.URL.Query().Get("id"))
	startDate := request.URL.Query().Get("start_date")
	endDate := request.URL.Query().Get("end_date")

	log.Println(roomID, startDate, endDate)

	sd, _ := time.Parse("2006-01-02", startDate)
	ed, _ := time.Parse("2006-01-02", endDate)

	var res = models.Reservation{
		StartDate: sd,
		EndDate:   ed,
		RoomID:    roomID,
	}

	m.App.Session.Put(request.Context(), "reservation", res)

	http.Redirect(writer, request, "/make-reservation", http.StatusTemporaryRedirect)
	return
}

type LoginForm struct {
	Email    string
	Password string
}

func (m *Repository) Login(writer http.ResponseWriter, request *http.Request) {

	var form = LoginForm{
		Email:    "",
		Password: "",
	}

	_ = render.Template(writer, *request, "login.page.gohtml", &models.TemplateData{
		Data: map[string]interface{}{
			"form": form,
		},
		Form: forms.New(request.PostForm),
	})
}

func (m *Repository) PostLogin(writer http.ResponseWriter, request *http.Request) {

	// Renew the token because it's a good practice
	_ = m.App.Session.RenewToken(request.Context())

	err := request.ParseForm()
	if err != nil {
		log.Println(err)
		return
	}

	form := forms.New(request.PostForm)
	form.Required("email", "password")
	form.IsEmail("email")

	formModel := LoginForm{
		Email:    request.Form.Get("email"),
		Password: request.Form.Get("password"),
	}

	if !form.Valid() {

		_ = render.Template(writer, *request, "login.page.gohtml", &models.TemplateData{
			Data: map[string]interface{}{
				"form": formModel,
			},
			Form: form,
		})
		return
	} else {
		id, _, err := m.DB.Authenticate(formModel.Email, formModel.Password)
		if err != nil {
			log.Println(err)
			m.App.Session.Put(request.Context(), "error", "Invalid login credentials")
			http.Redirect(writer, request, "/user/login", http.StatusSeeOther)
			return
		} else {
			m.App.Session.Put(request.Context(), "user_id", id)
			m.App.Session.Put(request.Context(), "flash", "Logged in successfully")
			http.Redirect(writer, request, "/", http.StatusSeeOther)
		}
	}
}

func (m *Repository) Logout(writer http.ResponseWriter, request *http.Request) {
	_ = m.App.Session.Destroy(request.Context())
	_ = m.App.Session.RenewToken(request.Context())
	m.App.Session.Put(request.Context(), "flash", "Logged out successfully")
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (m *Repository) AdminDashboard(writer http.ResponseWriter, request *http.Request) {
	_ = render.Template(writer, *request, "admin-dashboard.page.gohtml", &models.TemplateData{})
}

func (m *Repository) AdminReservations(writer http.ResponseWriter, request *http.Request) {
	reservations, err := m.DB.AllReservations()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	_ = render.Template(writer, *request, "admin-reservations.page.gohtml", &models.TemplateData{
		Data: map[string]interface{}{
			"reservations": reservations,
		},
	})
}

func (m *Repository) AdminReservationsNew(writer http.ResponseWriter, request *http.Request) {
	reservations, err := m.DB.AllNewReservations()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	_ = render.Template(writer, *request, "admin-reservations-new.page.gohtml", &models.TemplateData{
		Data: map[string]interface{}{
			"reservations": reservations,
		},
	})
}

func (m *Repository) AdminReservationsCalendar(writer http.ResponseWriter, request *http.Request) {

	now := time.Now()

	if request.URL.Query().Get("y") != "" && request.URL.Query().Get("m") != "" {
		year, _ := strconv.Atoi(request.URL.Query().Get("y"))
		month, _ := strconv.Atoi(request.URL.Query().Get("m"))
		now = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}

	next := now.AddDate(0, 1, 0)
	last := now.AddDate(0, -1, 0)

	nextMonth := next.Format("01")
	nextMonthYear := next.Format("2006")
	lastMonth := last.Format("01")
	lastMonthYear := last.Format("2006")

	currentYear, currentMonth, _ := now.Date()
	currentLocation := now.Location()
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	intMap := make(map[string]int)
	intMap["days_in_month"] = lastOfMonth.Day()

	rooms, err := m.DB.AllRooms()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	data := map[string]interface{}{
		"now":   now,
		"rooms": rooms,
	}

	for _, room := range rooms {
		reservationMap := make(map[string]int)
		blockMap := make(map[string]int)

		for d := firstOfMonth; d.After(lastOfMonth) == false; d = d.AddDate(0, 0, 1) {
			reservationMap[d.Format("2006-01-02")] = 0
			blockMap[d.Format("2006-01-02")] = 0
			blockMap[d.Format("2006-01-02")] = 0
		}

		restrictions, err := m.DB.GetRestrictionsForRoomByDate(room.ID, firstOfMonth, lastOfMonth)
		if err != nil {
			helpers.ServerError(writer, err)
			return
		}

		for _, restriction := range restrictions {
			for d := restriction.StartDate; d.Before(restriction.EndDate) == true; d = d.AddDate(0, 0, 1) {
				if restriction.ReservationID > 0 {
					reservationMap[d.Format("2006-01-02")] = restriction.ReservationID
				} else {
					blockMap[d.Format("2006-01-02")] = restriction.ID
				}
			}
		}

		data[fmt.Sprintf("reservation_map_%d", room.ID)] = reservationMap
		data[fmt.Sprintf("block_map_%d", room.ID)] = blockMap

		m.App.Session.Put(request.Context(), fmt.Sprintf("block_map_%d", room.ID), blockMap)
	}

	_ = render.Template(writer, *request, "admin-reservations-calendar.page.gohtml", &models.TemplateData{
		StringMap: map[string]string{
			"next_month":      nextMonth,
			"next_month_year": nextMonthYear,
			"last_month":      lastMonth,
			"last_month_year": lastMonthYear,
			"this_month":      now.Format("01"),
			"this_month_year": now.Format("2006"),
		},
		IntMap: intMap,
		Data:   data,
	})
}

func (m *Repository) AdminReservation(writer http.ResponseWriter, request *http.Request) {
	reservationID, err := strconv.Atoi(chi.URLParam(request, "id"))
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}
	src := chi.URLParam(request, "src")

	reservation, err := m.DB.GetReservationByID(reservationID)
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	_ = render.Template(writer, *request, "admin-reservation.page.gohtml", &models.TemplateData{
		Data: map[string]interface{}{
			"reservation": reservation,
		},
		StringMap: map[string]string{
			"src":   src,
			"year":  request.URL.Query().Get("y"),
			"month": request.URL.Query().Get("m"),
		},
		Form: forms.New(nil),
	})
}

func (m *Repository) AdminPostReservation(writer http.ResponseWriter, request *http.Request) {

	reservationID, err := strconv.Atoi(chi.URLParam(request, "id"))
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}
	src := chi.URLParam(request, "src")

	err = request.ParseForm()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	reservation, err := m.DB.GetReservationByID(reservationID)
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	reservation.FirstName = request.Form.Get("first_name")
	reservation.LastName = request.Form.Get("last_name")
	reservation.Phone = request.Form.Get("phone")
	reservation.Email = request.Form.Get("email")

	err = m.DB.UpdateReservation(reservation)
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	year := request.Form.Get("year")
	month := request.Form.Get("month")

	m.App.Session.Put(request.Context(), "flash", "Updated successfully")

	if year == "" {
		http.Redirect(writer, request, fmt.Sprintf("/admin/reservations/%s", src), http.StatusSeeOther)
	} else {
		http.Redirect(writer, request, fmt.Sprintf("/admin/reservations/calendar?y=%s&m=%s", year, month), http.StatusSeeOther)
	}
}

func (m *Repository) AdminProcessedReservation(writer http.ResponseWriter, request *http.Request) {
	reservationID, err := strconv.Atoi(chi.URLParam(request, "id"))
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	err = m.DB.UpdateProcessedForReservations(reservationID, 1)

	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (m *Repository) AdminDeleteReservation(writer http.ResponseWriter, request *http.Request) {
	reservationID, err := strconv.Atoi(chi.URLParam(request, "id"))
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	err = m.DB.DeleteReservation(reservationID)

	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (m *Repository) AdminPostReservationsCalendar(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	year, _ := strconv.Atoi(request.Form.Get("y"))
	month, _ := strconv.Atoi(request.Form.Get("m"))

	rooms, err := m.DB.AllRooms()
	if err != nil {
		helpers.ServerError(writer, err)
		return
	}

	form := forms.New(request.PostForm)

	a := form.Get("add_block")
	log.Println(a)
	b := form.Get("remove_block")
	log.Println(b)

	for _, room := range rooms {

		curMap := m.App.Session.Get(request.Context(), fmt.Sprintf("block_map_%d", room.ID)).(map[string]int)
		for name, value := range curMap {
			if val, ok := curMap[name]; ok {
				if val > 0 {
					if !form.Has(fmt.Sprintf("remove_block[%d][%s]", room.ID, name)) {
						log.Println("would delete block", value)
						err = m.DB.DeleteRoomRestriction(value)
						if err != nil {
							helpers.ServerError(writer, err)
							return
						}
					}
				}
			}
		}
	}

	for name, value := range request.PostForm["add_block"] {
		log.Println("name:", name, value)
		re := regexp.MustCompile(`(\d*):(\d*-\d*-\d*)`)
		match := re.FindStringSubmatch(value)
		addID, _ := strconv.Atoi(match[1])
		date, _ := time.Parse("2006-01-02", match[2])
		log.Println(addID, date)
		m.DB.InsertBlockForRoom(addID, date)
		if err != nil {
			helpers.ServerError(writer, err)
			return
		}
	}

	m.App.Session.Put(request.Context(), "flash", "Changes saved")
	http.Redirect(writer, request, fmt.Sprintf("/admin/reservations/calendar?y=%d&m=%02d", year, month), http.StatusSeeOther)
}
