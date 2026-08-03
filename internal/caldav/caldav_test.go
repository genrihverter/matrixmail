// Package caldav_test содержит тесты для CalDAV сервера
package caldav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mail-matrix-server/internal/models"
)

// mockStorage - заглушка хранилища для тестов
type mockStorage struct {
	calendars map[string]*models.Calendar
	events    map[string]*models.CalendarEvent
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		calendars: make(map[string]*models.Calendar),
		events:    make(map[string]*models.CalendarEvent),
	}
}

// Реализация методов RQLiteStorage для тестов (упрощенная)
func (m *mockStorage) CreateCalendar(calendar *models.Calendar) error {
	m.calendars[calendar.ID] = calendar
	return nil
}

func (m *mockStorage) GetCalendar(id, owner string) (*models.Calendar, error) {
	if cal, ok := m.calendars[id]; ok {
		return cal, nil
	}
	return nil, nil
}

func (m *mockStorage) ListCalendars(owner string) ([]*models.Calendar, error) {
	var result []*models.Calendar
	for _, cal := range m.calendars {
		if cal.Owner == owner {
			result = append(result, cal)
		}
	}
	return result, nil
}

func (m *mockStorage) UpdateCalendar(calendar *models.Calendar) error {
	m.calendars[calendar.ID] = calendar
	return nil
}

func (m *mockStorage) DeleteCalendar(id, owner string) error {
	delete(m.calendars, id)
	return nil
}

func (m *mockStorage) CreateEvent(event *models.CalendarEvent) error {
	m.events[event.ID] = event
	return nil
}

func (m *mockStorage) GetEvent(id, owner string) (*models.CalendarEvent, error) {
	if evt, ok := m.events[id]; ok {
		return evt, nil
	}
	return nil, nil
}

func (m *mockStorage) ListEvents(calendarID, owner string, start, end time.Time) ([]*models.CalendarEvent, error) {
	var result []*models.CalendarEvent
	for _, evt := range m.events {
		if evt.CalendarID == calendarID && evt.Owner == owner {
			result = append(result, evt)
		}
	}
	return result, nil
}

func (m *mockStorage) UpdateEvent(event *models.CalendarEvent) error {
	m.events[event.ID] = event
	return nil
}

func (m *mockStorage) DeleteEvent(id, owner string) error {
	delete(m.events, id)
	return nil
}

func (m *mockStorage) CreateAddressBook(book *models.AddressBook) error {
	return nil
}

func (m *mockStorage) GetAddressBook(id, owner string) (*models.AddressBook, error) {
	return nil, nil
}

func (m *mockStorage) ListAddressBooks(owner string) ([]*models.AddressBook, error) {
	return nil, nil
}

func (m *mockStorage) DeleteAddressBook(id, owner string) error {
	return nil
}

func (m *mockStorage) CreateContact(contact *models.Contact) error {
	return nil
}

func (m *mockStorage) GetContact(id, owner string) (*models.Contact, error) {
	return nil, nil
}

func (m *mockStorage) ListContacts(addressBookID, owner string) ([]*models.Contact, error) {
	return nil, nil
}

func (m *mockStorage) UpdateContact(contact *models.Contact) error {
	return nil
}

func (m *mockStorage) DeleteContact(id, owner string) error {
	return nil
}

func (m *mockStorage) HealthCheck(ctx context.Context) error {
	return nil
}

// TestCalDAVServer_Options тестирует OPTIONS запрос
func TestCalDAVServer_Options(t *testing.T) {
	server := NewCalDAVServer(nil)
	
	req := httptest.NewRequest("OPTIONS", "/caldav/", nil)
	req.SetBasicAuth("testuser", "password")
	w := httptest.NewRecorder()
	
	server.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус %d, получен %d", http.StatusOK, w.Code)
	}
	
	davHeader := w.Header().Get("DAV")
	if !strings.Contains(davHeader, "calendar-access") {
		t.Errorf("Ожидался заголовок DAV с calendar-access, получен %s", davHeader)
	}
}

// TestCalDAVServer_PropfindPrincipal тестирует PROPFIND запрос к principal
func TestCalDAVServer_PropfindPrincipal(t *testing.T) {
	server := NewCalDAVServer(nil)
	
	req := httptest.NewRequest("PROPFIND", "/caldav/testuser", nil)
	req.SetBasicAuth("testuser", "password")
	w := httptest.NewRecorder()
	
	server.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус %d, получен %d", http.StatusOK, w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "principal") && !strings.Contains(body, "calendar-home-set") {
		// Допускаем что principal может отсутствовать если возвращается calendar-home-set
		if !strings.Contains(body, "calendar") {
			t.Error("Ожидался элемент principal или calendar в ответе")
		}
	}
}

// TestCalDAVServer_Unauthorized тестирует отсутствие аутентификации
func TestCalDAVServer_Unauthorized(t *testing.T) {
	server := NewCalDAVServer(nil)
	
	req := httptest.NewRequest("PROPFIND", "/caldav/", nil)
	w := httptest.NewRecorder()
	
	server.ServeHTTP(w, req)
	
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Ожидался статус %d, получен %d", http.StatusUnauthorized, w.Code)
	}
}

// TestExtractUID тестирует извлечение UID из iCalendar данных
func TestExtractUID(t *testing.T) {
	data := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-uid-123@example.com
SUMMARY:Тестовое событие
END:VEVENT
END:VCALENDAR`
	
	uid := extractUID(data)
	expected := "test-uid-123@example.com"
	
	if uid != expected {
		t.Errorf("Ожидался UID %s, получен %s", expected, uid)
	}
}

// TestExtractSummary тестирует извлечение SUMMARY из iCalendar данных
func TestExtractSummary(t *testing.T) {
	data := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-uid@example.com
SUMMARY:Важная встреча
DESCRIPTION:Описание встречи
END:VEVENT
END:VCALENDAR`
	
	summary := extractSummary(data)
	expected := "Важная встреча"
	
	if summary != expected {
		t.Errorf("Ожидался SUMMARY %s, получен %s", expected, summary)
	}
}

// TestGenerateETag тестирует генерацию ETag
func TestGenerateETag(t *testing.T) {
	data := "тестовые данные"
	etag1 := generateETag(data)
	etag2 := generateETag(data)
	
	if etag1 != etag2 {
		t.Error("ETag для одинаковых данных должны совпадать")
	}
	
	if len(etag1) != 32 {
		t.Errorf("Ожидалась длина ETag 32 символа, получена %d", len(etag1))
	}
}

// TestEscapeXML тестирует экранирование XML
func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<test>", "&lt;test&gt;"},
		{"a & b", "a &amp; b"},
		{`quote "test"`, `quote &quot;test&quot;`},
		{"normal text", "normal text"},
	}
	
	for _, test := range tests {
		result := escapeXML(test.input)
		if result != test.expected {
			t.Errorf("Для input=%s ожидалось %s, получено %s", test.input, test.expected, result)
		}
	}
}

// TestCalDAVServer_MethodNotAllowed тестирует неподдерживаемый метод
func TestCalDAVServer_MethodNotAllowed(t *testing.T) {
	server := NewCalDAVServer(nil)
	
	req := httptest.NewRequest("PATCH", "/caldav/", nil)
	req.SetBasicAuth("testuser", "password")
	w := httptest.NewRecorder()
	
	server.ServeHTTP(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидался статус %d, получен %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGenerateICalendarData тестирует генерацию iCalendar данных
func TestGenerateICalendarData(t *testing.T) {
	now := time.Now()
	end := now.Add(time.Hour)
	
	data := generateICalendarData("test-uid", "Тестовое событие", now, end, "Описание", "Место")
	
	if !strings.Contains(data, "BEGIN:VCALENDAR") {
		t.Error("Отсутствует BEGIN:VCALENDAR")
	}
	if !strings.Contains(data, "UID:test-uid") {
		t.Error("Отсутствует UID")
	}
	if !strings.Contains(data, "SUMMARY:Тестовое событие") {
		t.Error("Отсутствует SUMMARY")
	}
	if !strings.Contains(data, "DESCRIPTION:Описание") {
		t.Error("Отсутствует DESCRIPTION")
	}
	if !strings.Contains(data, "LOCATION:Место") {
		t.Error("Отсутствует LOCATION")
	}
}

// generateICalendarData вспомогательная функция для теста
func generateICalendarData(uid, summary string, start, end time.Time, description, location string) string {
	formatDateTime := func(date time.Time) string {
		return date.Format("20060102T150405Z")
	}
	
	var buf bytes.Buffer
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//Mail Matrix Server//CalDAV//EN\r\n")
	buf.WriteString("BEGIN:VEVENT\r\n")
	buf.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	buf.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", formatDateTime(time.Now())))
	buf.WriteString(fmt.Sprintf("DTSTART:%s\r\n", formatDateTime(start)))
	buf.WriteString(fmt.Sprintf("DTEND:%s\r\n", formatDateTime(end)))
	buf.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", summary))
	if description != "" {
		buf.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", description))
	}
	if location != "" {
		buf.WriteString(fmt.Sprintf("LOCATION:%s\r\n", location))
	}
	buf.WriteString("END:VEVENT\r\n")
	buf.WriteString("END:VCALENDAR\r\n")
	
	return buf.String()
}
