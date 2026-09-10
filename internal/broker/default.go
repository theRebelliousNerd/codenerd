package broker

import (
	"net/http"
	"sync"
	"time"
)

// Meter owns the process-wide metering state: one ledger, one calibrator, one
// receipt buffer.
//
// It is process-scoped rather than per-client because "one ledger" is the whole
// point. codeNERD runs one workspace per process, so process scope and session
// scope coincide; a per-client ledger would reproduce the pre-broker situation
// with extra steps, where several budget authorities each believed they owned
// the window.
//
// The calibrator is shared for the same reason it is worth having at all: every
// client talking to a given model contributes observations to one ratio, so the
// perception client's cheap classification calls improve the session executor's
// admission accuracy for free.
type Meter struct {
	mu         sync.RWMutex
	ledger     *Ledger
	calibrator *Calibrator
	ring       *RingSink
	sink       ReceiptSink
	httpClient *http.Client

	reconciler *Reconciler

	// primaryModel is the model that serves the main conversation. Callers that
	// count text without knowing which model it is bound for resolve to this,
	// so a compressor sizing a context block uses the ratio learned from the
	// model that will actually read it.
	primaryModel string
}

// MeterConfig configures the process meter.
type MeterConfig struct {
	// Window is the model context window in tokens. Zero disables window
	// enforcement and is reported as zero headroom on every receipt rather
	// than silently passing.
	Window int
	// OutputReserve is held back for the response.
	OutputReserve int
	// Budgets are optional cumulative per-purpose token caps.
	Budgets map[Purpose]int64
	// ReceiptBuffer is how many recent receipts to retain. Zero uses a default.
	ReceiptBuffer int
	// SeedRatio overrides the estimator's starting chars-per-token ratio.
	SeedRatio float64
	// HTTPClient is used for provider counting endpoints.
	HTTPClient *http.Client
	// ExtraSink receives every receipt in addition to the built-in log and
	// ring sinks.
	ExtraSink ReceiptSink
	// PrimaryModel names the model serving the main conversation.
	PrimaryModel string
}

var (
	defaultMeterOnce sync.Once
	defaultMeter     *Meter
)

// Default returns the process meter, creating an unconfigured one if boot has
// not called Configure yet.
//
// An unconfigured meter still counts, still records, and still calibrates. It
// simply has no window to enforce, which is the honest state before config is
// loaded — and it is visible, because every receipt it emits reports a zero
// window rather than a satisfied check.
func Default() *Meter {
	defaultMeterOnce.Do(func() { defaultMeter = NewMeter(MeterConfig{}) })
	return defaultMeter
}

// Configure applies cfg to the process meter, replacing its limits. Recorded
// spend and learned calibration survive, because neither becomes untrue when a
// window size is discovered.
func Configure(cfg MeterConfig) {
	m := Default()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ledger.SetWindow(cfg.Window, cfg.OutputReserve)
	if len(cfg.Budgets) > 0 {
		// Rebuild the ledger's caps while preserving accumulated spend by
		// copying it across; a budget change must not silently zero the
		// balances it is being compared against.
		replacement := NewLedger(LedgerConfig{
			Window:        cfg.Window,
			OutputReserve: cfg.OutputReserve,
			Budgets:       cfg.Budgets,
		})
		for purpose, spend := range m.ledger.Accounts() {
			replacement.Record(purpose, spend)
		}
		m.ledger = replacement
	}
	if cfg.HTTPClient != nil {
		m.httpClient = cfg.HTTPClient
	}
	if cfg.ExtraSink != nil {
		m.sink = MultiSink{LogSink{}, m.ring, cfg.ExtraSink}
	}
	if cfg.PrimaryModel != "" {
		m.primaryModel = cfg.PrimaryModel
	}
}

// NewMeter builds an independent meter. Tests use this to avoid sharing the
// process meter; production uses Default.
func NewMeter(cfg MeterConfig) *Meter {
	ring := NewRingSink(cfg.ReceiptBuffer)

	var sink ReceiptSink = MultiSink{LogSink{}, ring}
	if cfg.ExtraSink != nil {
		sink = MultiSink{LogSink{}, ring, cfg.ExtraSink}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	calibrator := NewCalibrator()
	if cfg.SeedRatio > 0 {
		calibrator = NewCalibratorWithSeed(cfg.SeedRatio)
	}

	return &Meter{
		ledger: NewLedger(LedgerConfig{
			Window:        cfg.Window,
			OutputReserve: cfg.OutputReserve,
			Budgets:       cfg.Budgets,
		}),
		calibrator:   calibrator,
		reconciler:   NewReconciler(),
		ring:         ring,
		sink:         sink,
		httpClient:   httpClient,
		primaryModel: cfg.PrimaryModel,
	}
}

// Ledger returns the meter's ledger.
func (m *Meter) Ledger() *Ledger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ledger
}

// ProviderCreds is what a counter needs to reach a provider's counting
// endpoint. Providers with no such endpoint leave it zero and get the
// calibrating estimator.
type ProviderCreds struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// Calibrator returns the meter's shared calibrator.
//
// Exported for one reason worth stating: internal/context's token counter is
// built from this calibrator, and the property that a ratio learned here
// reaches a counter already handed out -- without reconstructing it, since the
// compressor holds one counter for a whole session -- is only checkable by
// feeding an observation in from outside the package.
//
// scripts/deadcode-budget.sh reports it unreachable because its only caller is
// a test in another package, which production-reachability analysis does not
// follow. It is recorded in the baseline for that reason rather than deleted.
func (m *Meter) Calibrator() *Calibrator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calibrator
}

// CounterFor returns the strongest counter available for the given provider.
//
// Anthropic gets the real count_tokens endpoint. Everything else gets the
// calibrating estimator, which is the same estimator Anthropic falls back to
// when its endpoint is unreachable — so a single shared calibrator keeps
// improving regardless of which path served any given request.
func (m *Meter) CounterFor(creds ProviderCreds) Counter {
	m.mu.RLock()
	cal := m.calibrator
	httpClient := m.httpClient
	m.mu.RUnlock()

	estimator := NewEstimatingCounter(cal)

	if creds.Provider == "anthropic" && creds.APIKey != "" && creds.BaseURL != "" {
		return NewAnthropicCounter(creds.APIKey, creds.BaseURL, httpClient, estimator)
	}
	return estimator
}

// ConfigFor builds a broker Config for a client with the given credentials.
func (m *Meter) ConfigFor(creds ProviderCreds) Config {
	m.mu.RLock()
	ledger := m.ledger
	sink := m.sink
	reconciler := m.reconciler
	m.mu.RUnlock()

	return Config{
		Provider:   creds.Provider,
		Model:      creds.Model,
		Counter:    m.CounterFor(creds),
		Ledger:     ledger,
		Sink:       sink,
		Reconciler: reconciler,
	}
}
