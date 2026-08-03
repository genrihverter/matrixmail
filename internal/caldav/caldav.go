// Package caldav реализует CalDAV сервер для работы с календарями
package caldav

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mail-matrix-server/internal/models"
	"mail-matrix-server/internal/rqlite"
)

// CalDAVServer представляет CalDAV сервер
type CalDAVServer struct {
	storage *rqlite.RQLiteStorage
}

// NewCalDAVServer создает новый CalDAV сервер
func NewCalDAVServer(storage *rqlite.RQLiteStorage) *CalDAVServer {
	return &CalDAVServer{
		storage: storage,
	}
}

// ServeHTTP обрабатывает HTTP запросы
func (s *CalDAVServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Получаем имя пользователя из Basic Auth
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="CalDAV Server"`)
		http.Error(w, "Требуется аутентификация", http.StatusUnauthorized)
		return
	}

	// В реальном приложении здесь должна быть проверка пароля
	_ = password

	path := r.URL.Path
	method := r.Method

	switch method {
	case "PROPFIND":
		s.handlePropfind(w, r, username, path)
	case "REPORT":
		s.handleReport(w, r, username, path)
	case "PUT":
		s.handlePut(w, r, username, path)
	case "GET":
		s.handleGet(w, r, username, path)
	case "DELETE":
		s.handleDelete(w, r, username, path)
	case "MKCOL":
		s.handleMkcol(w, r, username, path)
	case "OPTIONS":
		s.handleOptions(w, r)
	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

// handleOptions обрабатывает OPTIONS запрос
func (s *CalDAVServer) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2, calendar-access")
	w.Header().Set("Allow", "OPTIONS, PROPFIND, REPORT, PUT, GET, DELETE, MKCOL")
	w.WriteHeader(http.StatusOK)
}

// handlePropfind обрабатывает PROPFIND запрос для получения свойств
func (s *CalDAVServer) handlePropfind(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим XML запрос (упрощенно)
	_ = body

	// Определяем тип ресурса по пути
	if strings.HasSuffix(path, "/") || path == "/caldav" || path == "/caldav/"+username {
		// Запрос коллекции календарей
		s.handleCalendarCollection(w, username)
	} else if strings.Contains(path, "/calendar/") {
		// Запрос конкретного календаря или события
		parts := strings.Split(strings.TrimPrefix(path, "/caldav/"+username+"/calendar/"), "/")
		if len(parts) >= 1 {
			calendarID := parts[0]
			if len(parts) == 1 {
				s.handleCalendar(w, username, calendarID)
			} else {
				s.handleEvent(w, username, calendarID, parts[1])
			}
		}
	} else {
		// Корневой путь - возвращаем principal
		s.handlePrincipal(w, username)
	}
}

// handlePrincipal обрабатывает запрос к principal
func (s *CalDAVServer) handlePrincipal(w http.ResponseWriter, username string) {
	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/caldav/%s/</href>
    <propstat>
      <prop>
        <displayname>%s</displayname>
        <resourcetype><principal/></resourcetype>
        <principal-URL><href>/caldav/%s/</href></principal-URL>
        <calendar-home-set><href>/caldav/%s/calendar/</href></calendar-home-set>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`

	fmt.Fprintf(w, response, username, username, username, username)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleCalendarCollection обрабатывает запрос к коллекции календарей
func (s *CalDAVServer) handleCalendarCollection(w http.ResponseWriter, username string) {
	calendars, err := s.storage.ListCalendars(username)
	if err != nil {
		http.Error(w, "Ошибка получения календарей", http.StatusInternalServerError)
		return
	}

	var responses []string
	for _, cal := range calendars {
		responses = append(responses, fmt.Sprintf(`
  <response>
    <href>/caldav/%s/calendar/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
        <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, username, cal.ID, cal.Name, generateETag(cal.Name)))
	}

	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` + strings.Join(responses, "") + `
</multistatus>`

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleCalendar обрабатывает запрос к конкретному календарю
func (s *CalDAVServer) handleCalendar(w http.ResponseWriter, username, calendarID string) {
	calendar, err := s.storage.GetCalendar(calendarID, username)
	if err != nil {
		http.Error(w, "Календарь не найден", http.StatusNotFound)
		return
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/caldav/%s/calendar/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
        <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, username, calendar.ID, calendar.Name, generateETag(calendar.Name))

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleEvent обрабатывает запрос к событию
func (s *CalDAVServer) handleEvent(w http.ResponseWriter, username, calendarID, eventID string) {
	event, err := s.storage.GetEvent(eventID, username)
	if err != nil {
		http.Error(w, "Событие не найдено", http.StatusNotFound)
		return
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/caldav/%s/calendar/%s/%s</href>
    <propstat>
      <prop>
        <resourcetype/>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
        <getcontenttype>text/calendar</getcontenttype>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, username, calendarID, eventID, event.Summary, event.ETag)

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleReport обрабатывает REPORT запрос для поиска событий
func (s *CalDAVServer) handleReport(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим XML для определения типа отчета
	// В реальной реализации нужно парсить calendar-query или calendar-multiget
	_ = body

	// Извлекаем calendar ID из пути
	parts := strings.Split(strings.TrimPrefix(path, "/caldav/"+username+"/calendar/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	calendarID := parts[0]

	// Получаем события за период (упрощенно - все события)
	events, err := s.storage.ListEvents(calendarID, username, time.Now(), time.Now().AddDate(0, 1, 0))
	if err != nil {
		http.Error(w, "Ошибка получения событий", http.StatusInternalServerError)
		return
	}

	var responses []string
	for _, event := range events {
		responses = append(responses, fmt.Sprintf(`
  <response>
    <href>/caldav/%s/calendar/%s/%s</href>
    <propstat>
      <prop>
        <getetag>"%s"</getetag>
        <C:calendar-data>%s</C:calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, username, calendarID, event.ID, event.ETag, escapeXML(event.RawData)))
	}

	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` + strings.Join(responses, "") + `
</multistatus>`

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handlePut обрабатывает PUT запрос для создания/обновления события
func (s *CalDAVServer) handlePut(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Извлекаем calendar ID и event ID из пути
	parts := strings.Split(strings.TrimPrefix(path, "/caldav/"+username+"/calendar/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	calendarID := parts[0]
	eventID := strings.TrimSuffix(parts[1], ".ics")
	rawData := string(body)

	// Создаем или обновляем событие
	event := &models.CalendarEvent{
		ID:         eventID,
		UID:        extractUID(rawData),
		Owner:      username,
		CalendarID: calendarID,
		Summary:    extractSummary(rawData),
		RawData:    rawData,
	}

	// Проверяем If-None-Match для предотвращения перезаписи
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" {
		_, err := s.storage.GetEvent(eventID, username)
		if err == nil && ifNoneMatch != "*" {
			http.Error(w, "Конфликт версий", http.StatusPreconditionFailed)
			return
		}
	}

	var updateErr error
	_, err = s.storage.GetEvent(eventID, username)
	if err != nil {
		updateErr = s.storage.CreateEvent(event)
	} else {
		updateErr = s.storage.UpdateEvent(event)
	}

	if updateErr != nil {
		http.Error(w, "Ошибка сохранения события", http.StatusInternalServerError)
		return
	}

	// Устанавливаем ETag в ответ
	w.Header().Set("ETag", `"`+event.ETag+`"`)
	w.WriteHeader(http.StatusCreated)
}

// handleGet обрабатывает GET запрос для получения события
func (s *CalDAVServer) handleGet(w http.ResponseWriter, r *http.Request, username, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/caldav/"+username+"/calendar/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	eventID := strings.TrimSuffix(parts[1], ".ics")

	event, err := s.storage.GetEvent(eventID, username)
	if err != nil {
		http.Error(w, "Событие не найдено", http.StatusNotFound)
		return
	}

	// Проверяем If-None-Match
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == `"`+event.ETag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", `"`+event.ETag+`"`)
	fmt.Fprint(w, event.RawData)
}

// handleDelete обрабатывает DELETE запрос для удаления события
func (s *CalDAVServer) handleDelete(w http.ResponseWriter, r *http.Request, username, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/caldav/"+username+"/calendar/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	eventID := strings.TrimSuffix(parts[1], ".ics")

	err := s.storage.DeleteEvent(eventID, username)
	if err != nil {
		http.Error(w, "Ошибка удаления события", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMkcol обрабатывает MKCOL запрос для создания календаря
func (s *CalDAVServer) handleMkcol(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Извлекаем имя календаря из пути
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	calendarName := parts[len(parts)-1]
	calendarID := generateID()

	calendar := &models.Calendar{
		ID:          calendarID,
		Owner:       username,
		Name:        calendarName,
		Description: "Календарь создан через CalDAV",
		Color:       "#0066CC",
		Type:        "events",
		RawData:     string(body),
	}

	err = s.storage.CreateCalendar(calendar)
	if err != nil {
		http.Error(w, "Ошибка создания календаря", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// generateETag генерирует ETag
func generateETag(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// generateID генерирует уникальный ID
func generateID() string {
	return fmt.Sprintf("cal-%d", time.Now().UnixNano())
}

// extractUID извлекает UID из iCalendar данных
func extractUID(data string) string {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UID:") {
			return strings.TrimPrefix(line, "UID:")
		}
	}
	return fmt.Sprintf("uid-%d@local", time.Now().UnixNano())
}

// extractSummary извлекает SUMMARY из iCalendar данных
func extractSummary(data string) string {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "SUMMARY:") {
			return strings.TrimPrefix(line, "SUMMARY:")
		}
	}
	return "Без названия"
}

// escapeXML экранирует XML специальные символы
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// propfindRequest представляет структуру PROPFIND запроса
type propfindRequest struct {
	XMLName xml.Name `xml:"propfind"`
	Prop    struct {
		ResourceType struct{} `xml:"resourcetype"`
		DisplayName  struct{} `xml:"displayname"`
		GetETag      struct{} `xml:"getetag"`
	} `xml:"prop"`
}

// calendarQuery представляет структуру calendar-query отчета
type calendarQuery struct {
	XMLName xml.Name `xml:"calendar-query"`
	Filter  struct {
		CompFilter struct {
			Name       string `xml:"name,attr"`
			TimeFilter *struct {
				Start string `xml:"start,attr,omitempty"`
				End   string `xml:"end,attr,omitempty"`
			} `xml:"time-filter"`
		} `xml:"comp-filter"`
	} `xml:"filter"`
}

// Helper для конвертации строки в integer с обработкой ошибок
func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
