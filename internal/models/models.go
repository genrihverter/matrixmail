// Package models содержит модели данных для календарей и контактов
package models

import (
	"time"
)

// CalendarEvent представляет событие календаря (iCalendar формат)
type CalendarEvent struct {
	// Уникальный идентификатор события
	ID string `json:"id"`
	// UID события в формате iCalendar
	UID string `json:"uid"`
	// Владелец календаря (пользователь)
	Owner string `json:"owner"`
	// ID календаря
	CalendarID string `json:"calendar_id"`
	// Название события
	Summary string `json:"summary"`
	// Описание события
	Description string `json:"description"`
	// Место проведения
	Location string `json:"location"`
	// Время начала
	StartTime time.Time `json:"start_time"`
	// Время окончания
	EndTime time.Time `json:"end_time"`
	// Сырые данные в формате iCalendar (.ics)
	RawData string `json:"raw_data"`
	// Дата создания
	CreatedAt time.Time `json:"created_at"`
	// Дата последнего изменения
	UpdatedAt time.Time `json:"updated_at"`
	// ETag для синхронизации
	ETag string `json:"etag"`
}

// Calendar представляет календарь пользователя
type Calendar struct {
	// Уникальный идентификатор календаря
	ID string `json:"id"`
	// Владелец календаря
	Owner string `json:"owner"`
	// Название календаря
	Name string `json:"name"`
	// Описание календаря
	Description string `json:"description"`
	// Цвет календаря (для отображения в клиентах)
	Color string `json:"color"`
	// Тип календаря (events, tasks, etc.)
	Type string `json:"type"`
	// Сырые данные в формате iCalendar metadata
	RawData string `json:"raw_data"`
	// Дата создания
	CreatedAt time.Time `json:"created_at"`
	// Дата последнего изменения
	UpdatedAt time.Time `json:"updated_at"`
}

// Contact представляет контакт (vCard формат)
type Contact struct {
	// Уникальный идентификатор контакта
	ID string `json:"id"`
	// UID контакта в формате vCard
	UID string `json:"uid"`
	// Владелец адресной книги
	Owner string `json:"owner"`
	// ID адресной книги
	AddressBookID string `json:"address_book_id"`
	// Полное имя
	FullName string `json:"full_name"`
	// Email адреса
	Emails []string `json:"emails"`
	// Телефоны
	Phones []string `json:"phones"`
	// Сырые данные в формате vCard (.vcf)
	RawData string `json:"raw_data"`
	// Дата создания
	CreatedAt time.Time `json:"created_at"`
	// Дата последнего изменения
	UpdatedAt time.Time `json:"updated_at"`
	// ETag для синхронизации
	ETag string `json:"etag"`
}

// AddressBook представляет адресную книгу пользователя
type AddressBook struct {
	// Уникальный идентификатор адресной книги
	ID string `json:"id"`
	// Владелец адресной книги
	Owner string `json:"owner"`
	// Название адресной книги
	Name string `json:"name"`
	// Описание адресной книги
	Description string `json:"description"`
	// Сырые данные метаданных
	RawData string `json:"raw_data"`
	// Дата создания
	CreatedAt time.Time `json:"created_at"`
	// Дата последнего изменения
	UpdatedAt time.Time `json:"updated_at"`
}
