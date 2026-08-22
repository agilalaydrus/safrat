package domain

import "time"

// DefaultHajjRituals / DefaultUmrahRituals seed a fresh operator's ritual
// checklist — see spec section 3d. Name, description, required.
var DefaultHajjRituals = []struct{ Name, Description string }{
	{"Ihram", "Niat ihram di miqat"},
	{"Tawaf Qudum", "Tawaf kedatangan"},
	{"Sa'i", "Sa'i antara Shafa dan Marwah"},
	{"Wuquf Arafah", "Wukuf di Arafah — puncak ibadah Haji"},
	{"Mabit Muzdalifah", "Bermalam di Muzdalifah"},
	{"Mabit Mina", "Bermalam di Mina"},
	{"Lempar Jumrah", "Lempar jumrah di Mina"},
	{"Tawaf Ifadah", "Tawaf rukun Haji"},
	{"Tawaf Wada", "Tawaf perpisahan sebelum pulang"},
}

var DefaultUmrahRituals = []struct{ Name, Description string }{
	{"Ihram", "Niat ihram di miqat"},
	{"Tawaf Umrah", "Tawaf mengelilingi Ka'bah"},
	{"Sa'i", "Sa'i antara Shafa dan Marwah"},
	{"Tahallul", "Mencukur/memotong rambut"},
	{"Tawaf Wada", "Tawaf perpisahan sebelum pulang"},
}

type RitualTemplate struct {
	ID          string
	SeasonType  string
	Name        string
	Description string
	OrderNum    int32
	IsRequired  bool
}

type PilgrimRitualStatus struct {
	RitualID        string
	Name            string
	Description     string
	OrderNum        int32
	IsRequired      bool
	Completed       bool
	CompletedAt     *time.Time
	CompletedByName string
}

type RitualProgressItem struct {
	RitualID               string
	Name                   string
	OrderNum               int32
	TotalPilgrims          int32
	CompletedCount         int32
	IncompletePilgrimNames []string
}
