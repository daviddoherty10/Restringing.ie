package marketplace

import "time"

type Stringer struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Area            string `json:"area"`
	County          string `json:"county"`
	ApprovalStatus  string `json:"approval_status"`
	AcceptingOrders bool   `json:"accepting_orders"`
}

// ManagementProfile is private context. Public Stringer responses never expose
// user memberships; these flags describe the same checks enforced on each write.
type BusinessPermissions struct {
	EditProfile     bool `json:"edit_profile"`
	EditServices    bool `json:"edit_services"`
	ManageInventory bool `json:"manage_inventory"`
	ManageOrders    bool `json:"manage_orders"`
}
type ManagementProfile struct {
	Stringer
	MembershipRole string              `json:"membership_role"`
	Permissions    BusinessPermissions `json:"permissions"`
}
type OrderSummary struct {
	TotalOrders      int    `json:"total_orders"`
	ActiveOrders     int    `json:"active_orders"`
	CompletedOrders  int    `json:"completed_orders"`
	CancelledOrders  int    `json:"cancelled_orders"`
	UnpaidOrders     int    `json:"unpaid_orders"`
	BookedValueCents int64  `json:"booked_value_cents"`
	Currency         string `json:"currency"`
}

// Ratings are editorial catalogue attributes, not clinical or performance guarantees.
type Product struct {
	ID                   string `json:"id"`
	Brand                string `json:"brand"`
	Model                string `json:"model"`
	Material             string `json:"material"`
	Sport                string `json:"sport"`
	Description          string `json:"description"`
	Comfort              int    `json:"comfort"`
	Control              int    `json:"control"`
	Power                int    `json:"power"`
	Spin                 int    `json:"spin"`
	Durability           int    `json:"durability"`
	MaxPrestretchPercent int    `json:"max_prestretch_percent"`
	Active               bool   `json:"active"`
}

type Variant struct {
	ID                   string  `json:"id"`
	ProductID            string  `json:"product_id"`
	Brand                string  `json:"brand"`
	Model                string  `json:"model"`
	Description          string  `json:"description"`
	Material             string  `json:"material"`
	Sport                string  `json:"sport"`
	GaugeMM              float64 `json:"gauge_mm"`
	Colour               string  `json:"colour"`
	Comfort              int     `json:"comfort"`
	Control              int     `json:"control"`
	Power                int     `json:"power"`
	Spin                 int     `json:"spin"`
	Durability           int     `json:"durability"`
	MaxPrestretchPercent int     `json:"max_prestretch_percent"`
	InventoryID          string  `json:"inventory_id,omitempty"`
	MaterialPriceCents   int     `json:"material_price_cents"`
	Available            bool    `json:"available"`
}

type Racket struct {
	ID                    string `json:"id"`
	Brand                 string `json:"brand"`
	Model                 string `json:"model"`
	Sport                 string `json:"sport"`
	MinTension            int    `json:"min_tension_centi_kg"`
	MaxTension            int    `json:"max_tension_centi_kg"`
	Step                  int    `json:"step_centi_kg"`
	MaxDifference         int    `json:"max_difference_centi_kg"`
	AllowedKnots          []int  `json:"allowed_knots"`
	DifferentRequiresFour bool   `json:"different_tensions_require_four_knots"`
	StringLengthMM        int    `json:"string_length_mm"`
	TechnicalSource       string `json:"technical_source"`
	Verified              bool   `json:"verified"`
}

type Service struct {
	ID                     string      `json:"id"`
	Sport                  string      `json:"sport"`
	Active                 bool        `json:"active"`
	LabourCents            int         `json:"labour_cents"`
	OwnStrings             bool        `json:"own_strings"`
	OwnStringsCents        int         `json:"own_strings_cents"`
	Stencil                bool        `json:"stencil"`
	StencilCents           int         `json:"stencil_cents"`
	TwoKnots               bool        `json:"two_knots"`
	TwoKnotsCents          int         `json:"two_knots_cents"`
	FourKnots              bool        `json:"four_knots"`
	FourKnotsCents         int         `json:"four_knots_cents"`
	DifferentTensions      bool        `json:"different_tensions"`
	DifferentTensionsCents int         `json:"different_tensions_cents"`
	MinTension             int         `json:"min_tension_centi_kg"`
	MaxTension             int         `json:"max_tension_centi_kg"`
	Step                   int         `json:"step_centi_kg"`
	MaxDifference          int         `json:"max_difference_centi_kg"`
	Prestretch             map[int]int `json:"prestretch_options"`
}

type Booking struct {
	RacketModelID     string `json:"racket_model_id"`
	StringSource      string `json:"string_source"`
	VariantID         string `json:"variant_id,omitempty"`
	MainsTension      int    `json:"mains_tension_centi_kg"`
	CrossesTension    int    `json:"crosses_tension_centi_kg"`
	Knots             int    `json:"knots"`
	Stencil           bool   `json:"stencil"`
	PrestretchPercent int    `json:"prestretch_percent"`
	Instructions      string `json:"instructions"`
}

type Configuration struct {
	Booking
	RacketSnapshot string `json:"racket_snapshot"`
	StringSnapshot string `json:"string_snapshot"`
}

type PriceLine struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	AmountCents int    `json:"amount_cents"`
}

type Quote struct {
	ID            string        `json:"id"`
	StringerID    string        `json:"stringer_id"`
	Configuration Configuration `json:"configuration"`
	Breakdown     []PriceLine   `json:"breakdown"`
	TotalCents    int           `json:"total_cents"`
	Currency      string        `json:"currency"`
	ExpiresAt     time.Time     `json:"expires_at"`
	Fingerprint   string        `json:"-"`
}

type Order struct {
	ID            string        `json:"id"`
	CustomerID    string        `json:"customer_id"`
	StringerID    string        `json:"stringer_id"`
	QuoteID       string        `json:"quote_id"`
	Configuration Configuration `json:"configuration"`
	Breakdown     []PriceLine   `json:"breakdown"`
	TotalCents    int           `json:"total_cents"`
	Currency      string        `json:"currency"`
	PaymentStatus string        `json:"payment_status"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Events        []OrderEvent  `json:"events,omitempty"`
}

// Contact details are returned only by the business-scoped incoming-order route.
type OrderCustomer struct {
	Phone     string `json:"phone_number"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
type IncomingOrder struct {
	Order
	Customer OrderCustomer `json:"customer"`
}

type OrderEvent struct {
	From      *string   `json:"from_status"`
	To        string    `json:"to_status"`
	CreatedAt time.Time `json:"created_at"`
}

type Actor struct {
	ID    string
	Admin bool
}

type BookingOptions struct {
	StringerID    string  `json:"stringer_id"`
	Racket        Racket  `json:"racket"`
	Service       Service `json:"service"`
	MinTension    int     `json:"min_tension_centi_kg"`
	MaxTension    int     `json:"max_tension_centi_kg"`
	Step          int     `json:"step_centi_kg"`
	MaxDifference int     `json:"max_difference_centi_kg"`
	AllowedKnots  []int   `json:"allowed_knots"`
	Notice        string  `json:"notice"`
}

type Filters struct {
	VariantID string
	Query     string
	Material  string
	Brand     string
	Sport     string
	County    string
	GaugeMM   float64
	Page      int
	PageSize  int
}

type Page[T any] struct {
	Data     []T `json:"data"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type Problem struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (p *Problem) Error() string { return p.Code + ": " + p.Message }
func problem(status int, code, message string) error {
	return &Problem{Status: status, Code: code, Message: message}
}
func invalid(fields map[string]string) error {
	return &Problem{Status: 422, Code: "invalid_configuration", Message: "Check the supplied fields.", Fields: fields}
}
