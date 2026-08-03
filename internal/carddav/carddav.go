// Package carddav реализует CardDAV сервер для работы с контактами
package carddav

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mail-matrix-server/internal/models"
	"mail-matrix-server/internal/rqlite"
)

// CardDAVServer представляет CardDAV сервер
type CardDAVServer struct {
	storage *rqlite.RQLiteStorage
}

// NewCardDAVServer создает новый CardDAV сервер
func NewCardDAVServer(storage *rqlite.RQLiteStorage) *CardDAVServer {
	return &CardDAVServer{
		storage: storage,
	}
}

// ServeHTTP обрабатывает HTTP запросы
func (s *CardDAVServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Получаем имя пользователя из Basic Auth
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="CardDAV Server"`)
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
func (s *CardDAVServer) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2, addressbook")
	w.Header().Set("Allow", "OPTIONS, PROPFIND, REPORT, PUT, GET, DELETE, MKCOL")
	w.WriteHeader(http.StatusOK)
}

// handlePropfind обрабатывает PROPFIND запрос для получения свойств
func (s *CardDAVServer) handlePropfind(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим XML запрос (упрощенно)
	_ = body

	// Определяем тип ресурса по пути
	if strings.HasSuffix(path, "/") || path == "/carddav" || path == "/carddav/"+username {
		// Запрос коллекции адресных книг
		s.handleAddressBookCollection(w, username)
	} else if strings.Contains(path, "/addressbook/") {
		// Запрос конкретной адресной книги или контакта
		parts := strings.Split(strings.TrimPrefix(path, "/carddav/"+username+"/addressbook/"), "/")
		if len(parts) >= 1 {
			addressBookID := parts[0]
			if len(parts) == 1 {
				s.handleAddressBook(w, username, addressBookID)
			} else {
				s.handleContact(w, username, addressBookID, parts[1])
			}
		}
	} else {
		// Корневой путь - возвращаем principal
		s.handlePrincipal(w, username)
	}
}

// handlePrincipal обрабатывает запрос к principal
func (s *CardDAVServer) handlePrincipal(w http.ResponseWriter, username string) {
	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/carddav/%s/</href>
    <propstat>
      <prop>
        <displayname>%s</displayname>
        <resourcetype><principal/></resourcetype>
        <principal-URL><href>/carddav/%s/</href></principal-URL>
        <C:addressbook-home-set><href>/carddav/%s/addressbook/</href></C:addressbook-home-set>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`

	fmt.Fprintf(w, response, username, username, username, username)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleAddressBookCollection обрабатывает запрос к коллекции адресных книг
func (s *CardDAVServer) handleAddressBookCollection(w http.ResponseWriter, username string) {
	books, err := s.storage.ListAddressBooks(username)
	if err != nil {
		http.Error(w, "Ошибка получения адресных книг", http.StatusInternalServerError)
		return
	}

	var responses []string
	for _, book := range books {
		responses = append(responses, fmt.Sprintf(`
  <response>
    <href>/carddav/%s/addressbook/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:addressbook/></resourcetype>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, username, book.ID, book.Name, generateETag(book.Name)))
	}

	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">` + strings.Join(responses, "") + `
</multistatus>`

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleAddressBook обрабатывает запрос к конкретной адресной книге
func (s *CardDAVServer) handleAddressBook(w http.ResponseWriter, username, addressBookID string) {
	book, err := s.storage.GetAddressBook(addressBookID, username)
	if err != nil {
		http.Error(w, "Адресная книга не найдена", http.StatusNotFound)
		return
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/carddav/%s/addressbook/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:addressbook/></resourcetype>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, username, book.ID, book.Name, generateETag(book.Name))

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleContact обрабатывает запрос к контакту
func (s *CardDAVServer) handleContact(w http.ResponseWriter, username, addressBookID, contactID string) {
	contact, err := s.storage.GetContact(contactID, username)
	if err != nil {
		http.Error(w, "Контакт не найден", http.StatusNotFound)
		return
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/carddav/%s/addressbook/%s/%s</href>
    <propstat>
      <prop>
        <resourcetype/>
        <displayname>%s</displayname>
        <getetag>"%s"</getetag>
        <getcontenttype>text/vcard</getcontenttype>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, username, addressBookID, contactID, contact.FullName, contact.ETag)

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handleReport обрабатывает REPORT запрос для поиска контактов
func (s *CardDAVServer) handleReport(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим XML для определения типа отчета
	// В реальной реализации нужно парсить addressbook-query или addressbook-multiget
	_ = body

	// Извлекаем addressbook ID из пути
	parts := strings.Split(strings.TrimPrefix(path, "/carddav/"+username+"/addressbook/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	addressBookID := parts[0]

	// Получаем контакты (упрощенно - все контакты)
	contacts, err := s.storage.ListContacts(addressBookID, username)
	if err != nil {
		http.Error(w, "Ошибка получения контактов", http.StatusInternalServerError)
		return
	}

	var responses []string
	for _, contact := range contacts {
		responses = append(responses, fmt.Sprintf(`
  <response>
    <href>/carddav/%s/addressbook/%s/%s</href>
    <propstat>
      <prop>
        <getetag>"%s"</getetag>
        <C:address-data>%s</C:address-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, username, addressBookID, contact.ID, contact.ETag, escapeXML(contact.RawData)))
	}

	response := `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">` + strings.Join(responses, "") + `
</multistatus>`

	fmt.Fprint(w, response)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
}

// handlePut обрабатывает PUT запрос для создания/обновления контакта
func (s *CardDAVServer) handlePut(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Извлекаем addressbook ID и contact ID из пути
	parts := strings.Split(strings.TrimPrefix(path, "/carddav/"+username+"/addressbook/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	addressBookID := parts[0]
	contactID := strings.TrimSuffix(parts[1], ".vcf")
	rawData := string(body)

	// Создаем или обновляем контакт
	contact := &models.Contact{
		ID:            contactID,
		UID:           extractUID(rawData),
		Owner:         username,
		AddressBookID: addressBookID,
		FullName:      extractFN(rawData),
		Emails:        extractEmails(rawData),
		Phones:        extractPhones(rawData),
		RawData:       rawData,
	}

	// Проверяем If-None-Match для предотвращения перезаписи
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" {
		_, err := s.storage.GetContact(contactID, username)
		if err == nil && ifNoneMatch != "*" {
			http.Error(w, "Конфликт версий", http.StatusPreconditionFailed)
			return
		}
	}

	var updateErr error
	_, err = s.storage.GetContact(contactID, username)
	if err != nil {
		updateErr = s.storage.CreateContact(contact)
	} else {
		updateErr = s.storage.UpdateContact(contact)
	}

	if updateErr != nil {
		http.Error(w, "Ошибка сохранения контакта", http.StatusInternalServerError)
		return
	}

	// Устанавливаем ETag в ответ
	w.Header().Set("ETag", `"`+contact.ETag+`"`)
	w.WriteHeader(http.StatusCreated)
}

// handleGet обрабатывает GET запрос для получения контакта
func (s *CardDAVServer) handleGet(w http.ResponseWriter, r *http.Request, username, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/carddav/"+username+"/addressbook/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	contactID := strings.TrimSuffix(parts[1], ".vcf")

	contact, err := s.storage.GetContact(contactID, username)
	if err != nil {
		http.Error(w, "Контакт не найден", http.StatusNotFound)
		return
	}

	// Проверяем If-None-Match
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == `"`+contact.ETag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("ETag", `"`+contact.ETag+`"`)
	fmt.Fprint(w, contact.RawData)
}

// handleDelete обрабатывает DELETE запрос для удаления контакта
func (s *CardDAVServer) handleDelete(w http.ResponseWriter, r *http.Request, username, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/carddav/"+username+"/addressbook/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	contactID := strings.TrimSuffix(parts[1], ".vcf")

	err := s.storage.DeleteContact(contactID, username)
	if err != nil {
		http.Error(w, "Ошибка удаления контакта", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMkcol обрабатывает MKCOL запрос для создания адресной книги
func (s *CardDAVServer) handleMkcol(w http.ResponseWriter, r *http.Request, username, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Извлекаем имя адресной книги из пути
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}

	addressBookName := parts[len(parts)-1]
	addressBookID := generateID()

	book := &models.AddressBook{
		ID:          addressBookID,
		Owner:       username,
		Name:        addressBookName,
		Description: "Адресная книга создана через CardDAV",
		RawData:     string(body),
	}

	err = s.storage.CreateAddressBook(book)
	if err != nil {
		http.Error(w, "Ошибка создания адресной книги", http.StatusInternalServerError)
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
	return fmt.Sprintf("ab-%d", time.Now().UnixNano())
}

// extractUID извлекает UID из vCard данных
func extractUID(data string) string {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UID:") {
			return strings.TrimPrefix(line, "UID:")
		}
	}
	return fmt.Sprintf("uid-%d@local", time.Now().UnixNano())
}

// extractFN извлекает FN (Full Name) из vCard данных
func extractFN(data string) string {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "FN:") {
			return strings.TrimPrefix(line, "FN:")
		}
	}
	return "Без имени"
}

// extractEmails извлекает EMAIL из vCard данных
func extractEmails(data string) []string {
	var emails []string
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "EMAIL:") {
			email := strings.TrimPrefix(line, "EMAIL:")
			emails = append(emails, email)
		}
	}
	if len(emails) == 0 {
		emails = []string{}
	}
	return emails
}

// extractPhones извлекает TEL из vCard данных
func extractPhones(data string) []string {
	var phones []string
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "TEL:") {
			phone := strings.TrimPrefix(line, "TEL:")
			phones = append(phones, phone)
		}
	}
	if len(phones) == 0 {
		phones = []string{}
	}
	return phones
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

// addressbookQuery представляет структуру addressbook-query отчета
type addressbookQuery struct {
	XMLName xml.Name `xml:"addressbook-query"`
	Filter  struct {
		CompFilter struct {
			Name string `xml:"name,attr"`
		} `xml:"comp-filter"`
	} `xml:"filter"`
}
