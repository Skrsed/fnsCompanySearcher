package domain

type Address struct {
	RegionCode  string `json:"КодРегион"`
	Index       string `json:"Индекс"`
	FullAddress string `json:"АдресПолн"`
}

type Capital struct {
	TotalCapital string `json:"СумКап"`
}

type IndividualEntrepreneur struct {
	FullName         string  `json:"ФИОПолн"`
	INN              string  `json:"ИННФЛ"`
	OGRN             string  `json:"ОГРНИП"`
	RegistrationDate string  `json:"ДатаРег"`
	EntrepreneurType string  `json:"ВидИП"`
	Gender           string  `json:"Пол"`
	CitizenType      string  `json:"ВидГражд"`
	Status           string  `json:"Статус"`
	ClosureDate      string  `json:"ДатаПрекр"`
	Address          Address `json:"Адрес"`
	MainActivity     struct {
		Code string `json:"Код"`
		Text string `json:"Текст"`
	} `json:"ОснВидДеят"`
	Contacts string `json:"Контакты"`
}

type LegalEntity struct {
	INN              string  `json:"ИНН"`
	KPP              string  `json:"КПП"`
	OGRN             string  `json:"ОГРН"`
	RegistrationDate string  `json:"ДатаРег"`
	Status           string  `json:"Статус"`
	ClosureDate      string  `json:"ДатаПрекр,omitempty"`
	Capital          Capital `json:"Капитал,omitempty"`
	ShortName        string  `json:"НаимСокрЮЛ"`
	FullName         string  `json:"НаимПолнЮЛ"`
	Address          Address `json:"Адрес"`
	CEO              struct {
		FullName string `json:"ФИОПолн"`
		INN      string `json:"ИННФЛ"`
	} `json:"Руководитель"`
	MainActivity struct {
		Code string `json:"Код"`
		Text string `json:"Текст"`
	} `json:"ОснВидДеят"`
	Phone    string   `json:"НомТел"`
	Email    string   `json:"E-mail"`
	Contacts string   `json:"Контакты"`
	Finances []string `json:"Финансы"`
}

// type Item struct {
// 	Company struct {
// 		INN      string `json:"ИНН|ИННФЛ"`
// 		Contacts string `json:"Контакты"`
// 	} `json:"ИП|НО|ЮЛ"`
// }

type Company struct {
	OGRN     string `json:"ОГРН"`
	Contacts string `json:"Контакты"`
	Finances string `json:"Финансы"`
	JSONB    []byte `json:"JSONB"`
}

type Item struct {
	IndividualEntrepreneur *IndividualEntrepreneur `json:"ИП,omitempty"`
	LegalEntity            *LegalEntity            `json:"ЮЛ,omitempty"`
}

type Response struct {
	Items []Item `json:"items"`
}
