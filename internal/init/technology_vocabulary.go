package init

import (
	"sort"
	"strings"
)

// Technology vocabulary: the one table of packages init recognizes as a
// technology, per ecosystem, mapped to the technology key the agent catalog
// (internal/orient/ecosystem_agents.mg profile_agent) creates shard agents
// for. Every dependency scanner reads it -- go.mod and go.sum, package.json
// and the npm lock files, requirements.txt, pyproject.toml and the Python lock
// files, Cargo.toml and Cargo.lock -- and boot asserts it as the import facts
// the kernel matches a file's imports against (TechnologyImportRoots,
// technology_import_exact / technology_import_prefix in
// policy/shard_experts.mg). So the key a manifest created an agent for is the
// key a file's import names. Before this each scanner kept its own local
// table and they disagreed (a yarn.lock reported "solid-js" where
// package-lock.json reported "solid").

// Ecosystem names a package namespace.
type Ecosystem string

const (
	EcosystemGo     Ecosystem = "go"
	EcosystemNPM    Ecosystem = "npm"
	EcosystemPython Ecosystem = "python"
	EcosystemRust   Ecosystem = "rust"
)

// technologyVocabulary maps, per ecosystem, a package name as its manifest
// spells it (a Go module path, an npm name, a Python distribution, a crate)
// to its technology key.
var technologyVocabulary = map[Ecosystem]map[string]string{
	EcosystemGo: {
		"github.com/go-rod/rod":               "rod",
		"github.com/chromedp/chromedp":        "chromedp",
		"github.com/playwright-community":     "playwright",
		"google/mangle":                       "mangle",
		"codeberg.org/TauCeti/mangle-go":      "mangle",
		"github.com/sashabaranov/go-openai":   "openai",
		"github.com/anthropics/anthropic":     "anthropic",
		"github.com/charmbracelet/bubbletea":  "bubbletea",
		"github.com/spf13/cobra":              "cobra",
		"github.com/gin-gonic/gin":            "gin",
		"github.com/labstack/echo":            "echo",
		"github.com/gofiber/fiber":            "fiber",
		"gorm.io/gorm":                        "gorm",
		"github.com/jmoiron/sqlx":             "sqlx",
		"database/sql":                        "sql",
		"github.com/gorilla/mux":              "gorilla",
		"net/http":                            "http",
		"github.com/arangodb/go-driver":       "arangodb",
		"google.golang.org/adk":               "adk",
		"github.com/a2aserver/a2a-go":         "a2a",
		"github.com/stretchr/testify":         "testify",
		"github.com/google/uuid":              "uuid",
		"go.uber.org/zap":                     "zap",
		"github.com/sirupsen/logrus":          "logrus",
		"github.com/pkg/errors":               "errors",
		"golang.org/x/sync":                   "sync",
		"golang.org/x/crypto":                 "crypto",
		"github.com/prometheus/client_golang": "prometheus",
		"github.com/go-playground/validator":  "validator",
		"github.com/dgrijalva/jwt-go":         "jwt",
		"github.com/golang-jwt/jwt":           "jwt",
		"github.com/redis/go-redis":           "redis",
		"github.com/go-redis/redis":           "redis",
		"go.mongodb.org/mongo-driver":         "mongodb",
		"github.com/aws/aws-sdk-go":           "aws-sdk",
		"cloud.google.com/go":                 "gcp-sdk",
		"github.com/Azure/azure-sdk-for-go":   "azure-sdk",
		"k8s.io/client-go":                    "kubernetes",
		"github.com/hashicorp/consul":         "consul",
		"github.com/hashicorp/vault":          "vault",
		"github.com/nats-io/nats.go":          "nats",
		"github.com/segmentio/kafka-go":       "kafka",
		"github.com/streadway/amqp":           "rabbitmq",
		"github.com/graphql-go/graphql":       "graphql",
		"github.com/99designs/gqlgen":         "gqlgen",
		"google.golang.org/grpc":              "grpc",
		"github.com/grpc-ecosystem":           "grpc-ecosystem",
	},
	EcosystemNPM: {
		"puppeteer":             "puppeteer",
		"playwright":            "playwright",
		"openai":                "openai",
		"@anthropic-ai/sdk":     "anthropic",
		"react":                 "react",
		"vue":                   "vue",
		"next":                  "nextjs",
		"express":               "express",
		"fastify":               "fastify",
		"prisma":                "prisma",
		"@prisma/client":        "prisma",
		"typeorm":               "typeorm",
		"angular":               "angular",
		"@angular/core":         "angular",
		"svelte":                "svelte",
		"solid-js":              "solid",
		"@nestjs/core":          "nestjs",
		"koa":                   "koa",
		"typescript":            "typescript",
		"webpack":               "webpack",
		"vite":                  "vite",
		"jest":                  "jest",
		"vitest":                "vitest",
		"mocha":                 "mocha",
		"cypress":               "cypress",
		"eslint":                "eslint",
		"@babel/core":           "babel",
		"esbuild":               "esbuild",
		"rollup":                "rollup",
		"prettier":              "prettier",
		"axios":                 "axios",
		"lodash":                "lodash",
		"moment":                "moment",
		"dayjs":                 "dayjs",
		"rxjs":                  "rxjs",
		"socket.io":             "socket.io",
		"mongoose":              "mongoose",
		"sequelize":             "sequelize",
		"redis":                 "redis",
		"@aws-sdk/client-s3":    "aws-sdk",
		"@google-cloud/storage": "gcp-sdk",
		"nuxt":                  "nuxt",
		"@nuxt/kit":             "nuxt",
		"gatsby":                "gatsby",
		"htmx.org":              "htmx",
		"tailwindcss":           "tailwind",
		"@emotion/react":        "emotion",
		"styled-components":     "styled-components",
	},
	EcosystemPython: {
		"django":       "django",
		"flask":        "flask",
		"fastapi":      "fastapi",
		"celery":       "celery",
		"redis":        "redis",
		"sqlalchemy":   "sqlalchemy",
		"pytest":       "pytest",
		"numpy":        "numpy",
		"pandas":       "pandas",
		"tensorflow":   "tensorflow",
		"torch":        "pytorch",
		"boto3":        "aws-sdk",
		"google-cloud": "gcp-sdk",
		"azure":        "azure-sdk",
		"pydantic":     "pydantic",
		"httpx":        "httpx",
		"aiohttp":      "aiohttp",
	},
	EcosystemRust: {
		"tokio":      "tokio",
		"async-std":  "async-std",
		"hyper":      "hyper",
		"actix-web":  "actix-web",
		"axum":       "axum",
		"rocket":     "rocket",
		"warp":       "warp",
		"diesel":     "diesel",
		"sqlx":       "sqlx",
		"serde":      "serde",
		"tracing":    "tracing",
		"clap":       "clap",
		"reqwest":    "reqwest",
		"tonic":      "tonic",
		"prost":      "prost",
		"redis":      "redis",
		"lapin":      "lapin",
		"rdkafka":    "kafka",
		"aws-sdk-s3": "aws-sdk",
	},
}

// pythonImportModules are the distributions whose import name is not their
// distribution name with dashes as underscores.
var pythonImportModules = map[string]string{
	"google-cloud": "google.cloud",
}

// technologyFor is a package's technology key in an ecosystem's vocabulary.
func technologyFor(eco Ecosystem, pkg string) (string, bool) {
	key, ok := technologyVocabulary[eco][pkg]
	return key, ok
}

// vocabularyPackages is an ecosystem's package names, sorted, so a scan that
// tests each name against manifest text reports in a stable order.
func vocabularyPackages(eco Ecosystem) []string {
	names := make([]string, 0, len(technologyVocabulary[eco]))
	for name := range technologyVocabulary[eco] {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ImportRoot is how a package's imports are spelled in source: an import
// equal to Exact, or starting with Prefix (Exact plus the ecosystem's
// separator), names Technology.
type ImportRoot struct {
	Ecosystem  Ecosystem
	Exact      string
	Prefix     string
	Technology string
}

// TechnologyImportRoots is the vocabulary as import spellings, the facts boot
// asserts (internal/system/shard_expert_facts.go):
//
//	Go      module path "github.com/x/y"  -> "github.com/x/y", "github.com/x/y/..."
//	npm     package "@scope/name"         -> "@scope/name", "@scope/name/..."
//	Python  distribution "foo-bar"        -> module "foo_bar", "foo_bar...."
//	Rust    crate "foo-bar"               -> path "foo_bar", "foo_bar::..."
//
// The separator keeps a root from matching a longer name ("react" does not
// match "react-dom"; "tokio" does not match "tokio_util").
func TechnologyImportRoots() []ImportRoot {
	var out []ImportRoot
	for _, eco := range []Ecosystem{EcosystemGo, EcosystemNPM, EcosystemPython, EcosystemRust} {
		for _, pkg := range vocabularyPackages(eco) {
			tech := technologyVocabulary[eco][pkg]
			exact, sep := pkg, "/"
			switch eco {
			case EcosystemPython:
				exact, sep = strings.ReplaceAll(strings.ToLower(pkg), "-", "_"), "."
				if module, ok := pythonImportModules[pkg]; ok {
					exact = module
				}
			case EcosystemRust:
				exact, sep = strings.ReplaceAll(pkg, "-", "_"), "::"
			}
			out = append(out, ImportRoot{Ecosystem: eco, Exact: exact, Prefix: exact + sep, Technology: tech})
		}
	}
	return out
}
