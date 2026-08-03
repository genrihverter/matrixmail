// TypeScript интерфейс для работы с CalDAV/CardDAV сервером
// Этот файл предоставляет типизированный клиент для взаимодействия с сервером

/**
 * Интерфейс для события календаря в формате iCalendar
 */
export interface CalendarEvent {
  id: string;
  uid: string;
  calendarId: string;
  summary: string;
  description?: string;
  location?: string;
  startTime: Date;
  endTime: Date;
  rawData: string;
  etag: string;
  createdAt: Date;
  updatedAt: Date;
}

/**
 * Интерфейс для календаря
 */
export interface Calendar {
  id: string;
  owner: string;
  name: string;
  description?: string;
  color: string;
  type: 'events' | 'tasks' | 'journal';
  createdAt: Date;
  updatedAt: Date;
}

/**
 * Интерфейс для контакта в формате vCard
 */
export interface Contact {
  id: string;
  uid: string;
  addressBookId: string;
  fullName: string;
  emails: string[];
  phones: string[];
  rawData: string;
  etag: string;
  createdAt: Date;
  updatedAt: Date;
}

/**
 * Интерфейс для адресной книги
 */
export interface AddressBook {
  id: string;
  owner: string;
  name: string;
  description?: string;
  createdAt: Date;
  updatedAt: Date;
}

/**
 * Конфигурация клиента CalDAV/CardDAV
 */
export interface DavClientConfig {
  baseUrl: string;
  username: string;
  password: string;
  timeout?: number;
}

/**
 * Результат запроса с ETag для синхронизации
 */
export interface DavResponse<T> {
  data: T;
  etag?: string;
  status: number;
}

/**
 * Параметры для запроса событий календаря
 */
export interface CalendarQueryParams {
  calendarId: string;
  startDate?: Date;
  endDate?: Date;
}

/**
 * Параметры для запроса контактов
 */
export interface AddressBookQueryParams {
  addressBookId: string;
  filter?: {
    fullName?: string;
    email?: string;
  };
}

/**
 * Класс клиента для работы с CalDAV/CardDAV сервером
 * Пример использования:
 * 
 * ```typescript
 * const client = new DavClient({
 *   baseUrl: 'http://localhost:8080',
 *   username: 'user@example.com',
 *   password: 'password'
 * });
 * 
 * // Получить список календарей
 * const calendars = await client.listCalendars();
 * 
 * // Создать событие
 * const event = await client.createEvent('cal-1', {
 *   summary: 'Встреча',
 *   startTime: new Date(),
 *   endTime: new Date(Date.now() + 3600000),
 *   rawData: 'BEGIN:VEVENT...'
 * });
 * 
 * // Получить список контактов
 * const contacts = await client.listContacts('ab-1');
 * ```
 */
export class DavClient {
  private config: DavClientConfig;
  private authHeader: string;

  constructor(config: DavClientConfig) {
    this.config = config;
    // Создаем Basic Auth заголовок
    const credentials = btoa(`${config.username}:${config.password}`);
    this.authHeader = `Basic ${credentials}`;
  }

  /**
   * Выполняет HTTP запрос к серверу
   */
  private async request<T>(
    method: string,
    path: string,
    body?: string,
    headers: Record<string, string> = {}
  ): Promise<DavResponse<T>> {
    const url = `${this.config.baseUrl}${path}`;
    
    const response = await fetch(url, {
      method,
      headers: {
        'Authorization': this.authHeader,
        'Content-Type': 'application/xml; charset=utf-8',
        ...headers,
      },
      body,
    });

    if (!response.ok) {
      throw new Error(`HTTP ошибка: ${response.status} ${response.statusText}`);
    }

    const etag = response.headers.get('ETag') || undefined;
    
    let data: T;
    const contentType = response.headers.get('Content-Type');
    
    if (contentType?.includes('application/xml') || contentType?.includes('text/xml')) {
      const text = await response.text();
      data = this.parseXml(text) as T;
    } else if (contentType?.includes('application/json')) {
      data = await response.json();
    } else {
      data = await response.text() as unknown as T;
    }

    return { data, etag, status: response.status };
  }

  /**
   * Парсит XML ответ (упрощенная реализация)
   */
  private parseXml(xml: string): unknown {
    // В реальной реализации используйте DOMParser или библиотеку типа fast-xml-parser
    console.warn('XML парсинг не реализован полностью');
    return { raw: xml };
  }

  // ==================== CalDAV методы ====================

  /**
   * Получает список календарей пользователя
   */
  async listCalendars(): Promise<DavResponse<Calendar[]>> {
    const path = `/caldav/${this.config.username}/`;
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:resourcetype/>
    <d:displayname/>
    <d:getetag/>
    <c:supported-calendar-component-set/>
  </d:prop>
</d:propfind>`;
    
    return this.request<Calendar[]>('PROPFIND', path, body);
  }

  /**
   * Получает события из календаря за период
   */
  async getEvents(params: CalendarQueryParams): Promise<DavResponse<CalendarEvent[]>> {
    const path = `/caldav/${this.config.username}/calendar/${params.calendarId}/`;
    
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag/>
    <c:calendar-data/>
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCALENDAR">
      <c:comp-filter name="VEVENT">
        ${params.startDate && params.endDate ? `
        <c:time-range start="${params.startDate.toISOString()}" end="${params.endDate.toISOString()}"/>
        ` : ''}
      </c:comp-filter>
    </c:comp-filter>
  </c:filter>
</c:calendar-query>`;

    return this.request<CalendarEvent[]>('REPORT', path, body);
  }

  /**
   * Создает или обновляет событие в календаре
   */
  async createEvent(
    calendarId: string,
    eventData: {
      summary: string;
      startTime: Date;
      endTime: Date;
      rawData: string;
      description?: string;
      location?: string;
    },
    eventId?: string,
    ifNoneMatch?: boolean
  ): Promise<DavResponse<CalendarEvent>> {
    const id = eventId || `evt-${Date.now()}`;
    const path = `/caldav/${this.config.username}/calendar/${calendarId}/${id}.ics`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'text/calendar; charset=utf-8',
    };

    if (ifNoneMatch) {
      headers['If-None-Match'] = '*';
    }

    return this.request<CalendarEvent>('PUT', path, eventData.rawData, headers);
  }

  /**
   * Получает конкретное событие
   */
  async getEvent(calendarId: string, eventId: string): Promise<DavResponse<CalendarEvent>> {
    const path = `/caldav/${this.config.username}/calendar/${calendarId}/${eventId}.ics`;
    return this.request<CalendarEvent>('GET', path);
  }

  /**
   * Удаляет событие
   */
  async deleteEvent(calendarId: string, eventId: string): Promise<void> {
    const path = `/caldav/${this.config.username}/calendar/${calendarId}/${eventId}.ics`;
    await this.request<void>('DELETE', path);
  }

  /**
   * Создает новый календарь
   */
  async createCalendar(name: string, color: string = '#0066CC'): Promise<DavResponse<Calendar>> {
    const path = `/caldav/${this.config.username}/calendar/${name}/`;
    
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<d:mkcol xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:set>
    <d:prop>
      <d:resourcetype>
        <d:collection/>
        <c:calendar/>
      </d:resourcetype>
      <d:displayname>${name}</d:displayname>
      <c:supported-calendar-component-set>
        <c:comp name="VEVENT"/>
      </c:supported-calendar-component-set>
    </d:prop>
  </d:set>
</d:mkcol>`;

    return this.request<Calendar>('MKCOL', path, body);
  }

  // ==================== CardDAV методы ====================

  /**
   * Получает список адресных книг пользователя
   */
  async listAddressBooks(): Promise<DavResponse<AddressBook[]>> {
    const path = `/carddav/${this.config.username}/`;
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">
  <d:prop>
    <d:resourcetype/>
    <d:displayname/>
    <d:getetag/>
  </d:prop>
</d:propfind>`;
    
    return this.request<AddressBook[]>('PROPFIND', path, body);
  }

  /**
   * Получает контакты из адресной книги
   */
  async getContacts(params: AddressBookQueryParams): Promise<DavResponse<Contact[]>> {
    const path = `/carddav/${this.config.username}/addressbook/${params.addressBookId}/`;
    
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<c:addressbook-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">
  <d:prop>
    <d:getetag/>
    <c:address-data/>
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCARD"/>
  </c:filter>
</c:addressbook-query>`;

    return this.request<Contact[]>('REPORT', path, body);
  }

  /**
   * Создает или обновляет контакт
   */
  async createContact(
    addressBookId: string,
    contactData: {
      fullName: string;
      emails: string[];
      phones: string[];
      rawData: string;
    },
    contactId?: string,
    ifNoneMatch?: boolean
  ): Promise<DavResponse<Contact>> {
    const id = contactId || `cnt-${Date.now()}`;
    const path = `/carddav/${this.config.username}/addressbook/${addressBookId}/${id}.vcf`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'text/vcard; charset=utf-8',
    };

    if (ifNoneMatch) {
      headers['If-None-Match'] = '*';
    }

    return this.request<Contact>('PUT', path, contactData.rawData, headers);
  }

  /**
   * Получает конкретный контакт
   */
  async getContact(addressBookId: string, contactId: string): Promise<DavResponse<Contact>> {
    const path = `/carddav/${this.config.username}/addressbook/${addressBookId}/${contactId}.vcf`;
    return this.request<Contact>('GET', path);
  }

  /**
   * Удаляет контакт
   */
  async deleteContact(addressBookId: string, contactId: string): Promise<void> {
    const path = `/carddav/${this.config.username}/addressbook/${addressBookId}/${contactId}.vcf`;
    await this.request<void>('DELETE', path);
  }

  /**
   * Создает новую адресную книгу
   */
  async createAddressBook(name: string): Promise<DavResponse<AddressBook>> {
    const path = `/carddav/${this.config.username}/addressbook/${name}/`;
    
    const body = `<?xml version="1.0" encoding="UTF-8"?>
<d:mkcol xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">
  <d:set>
    <d:prop>
      <d:resourcetype>
        <d:collection/>
        <c:addressbook/>
      </d:resourcetype>
      <d:displayname>${name}</d:displayname>
    </d:prop>
  </d:set>
</d:mkcol>`;

    return this.request<AddressBook>('MKCOL', path, body);
  }
}

// Экспорт утилит для работы с iCalendar и vCard данными

/**
 * Генерирует iCalendar событие из объекта
 */
export function generateICalendar(event: {
  uid: string;
  summary: string;
  startTime: Date;
  endTime: Date;
  description?: string;
  location?: string;
}): string {
  const formatDateTime = (date: Date) => {
    return date.toISOString().replace(/[-:]/g, '').split('.')[0] + 'Z';
  };

  return [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Mail Matrix Server//CalDAV//EN',
    'BEGIN:VEVENT',
    `UID:${event.uid}`,
    `DTSTAMP:${formatDateTime(new Date())}`,
    `DTSTART:${formatDateTime(event.startTime)}`,
    `DTEND:${formatDateTime(event.endTime)}`,
    `SUMMARY:${event.summary}`,
    event.description ? `DESCRIPTION:${event.description}` : '',
    event.location ? `LOCATION:${event.location}` : '',
    'END:VEVENT',
    'END:VCALENDAR',
  ].filter(Boolean).join('\r\n');
}

/**
 * Генерирует vCard контакт из объекта
 */
export function generateVCard(contact: {
  uid: string;
  fullName: string;
  emails: string[];
  phones: string[];
}): string {
  return [
    'BEGIN:VCARD',
    'VERSION:3.0',
    `UID:${contact.uid}`,
    `FN:${contact.fullName}`,
    `N:;${contact.fullName};;;`,
    ...contact.emails.map(email => `EMAIL:${email}`),
    ...contact.phones.map(phone => `TEL:${phone}`),
    'END:VCARD',
  ].join('\r\n');
}

/**
 * Парсит iCalendar данные (упрощенно)
 */
export function parseICalendar(data: string): Partial<CalendarEvent> {
  const lines = data.split('\r\n');
  const result: Partial<CalendarEvent> = {};

  for (const line of lines) {
    if (line.startsWith('SUMMARY:')) {
      result.summary = line.slice(8);
    } else if (line.startsWith('UID:')) {
      result.uid = line.slice(4);
    } else if (line.startsWith('DESCRIPTION:')) {
      result.description = line.slice(12);
    } else if (line.startsWith('LOCATION:')) {
      result.location = line.slice(9);
    }
  }

  return result;
}

/**
 * Парсит vCard данные (упрощенно)
 */
export function parseVCard(data: string): Partial<Contact> {
  const lines = data.split('\r\n');
  const result: Partial<Contact> = {
    emails: [],
    phones: [],
  };

  for (const line of lines) {
    if (line.startsWith('FN:')) {
      result.fullName = line.slice(3);
    } else if (line.startsWith('UID:')) {
      result.uid = line.slice(4);
    } else if (line.startsWith('EMAIL:')) {
      result.emails?.push(line.slice(6));
    } else if (line.startsWith('TEL:')) {
      result.phones?.push(line.slice(4));
    }
  }

  return result;
}
