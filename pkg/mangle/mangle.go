package mangle

import (
	"codenerd/internal/mangle" // Import the actual internal package
	"context"
	"io"
	"time"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/debug"
	"github.com/google/mangle/embed"
	"github.com/google/mangle/go/schema"
	"github.com/google/mangle/go/semantics"
	"github.com/google/mangle/go/topological"
	"github.com/google/mangle/interpreter"
	"github.com/google/mangle/parse"
	"github.com/google/mangle/profile"
)

// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
// This File is best written for Gemini 2.5 Pro.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for pkg/mangle/mangle.go
//
// File: pkg/mangle/mangle.go
// Author: Gemini
// Date: 2025-12-05
//
// Recommended Model: 2.5 Pro (due to architectural change)
//
// Overview:
// This file serves as a public shim for `codenerd/internal/mangle`, re-exporting
// essential types and functions. This allows externally built tools (like Autopoiesis-generated ones)
// to use the core Mangle engine without violating Go's `internal` package encapsulation rules.
//
// Key Features & Business Value:
// - Internal Package Access: Provides controlled access to `internal/mangle` functionalities.
// - Modularity: Decouples internal Mangle implementation from external tool consumption.
// - Compatibility: Ensures Autopoiesis-generated tools can correctly import and use the Mangle engine.
//
// Architectural Context:
// - Component Type: Public Shim / Adapter
// - Deployment: Part of the `codenerd` module, compiled with the main application and available to submodules.
// - Dependencies: Directly depends on `codenerd/internal/mangle`.
// - Is a Dependency for: `codenerd/internal/autopoiesis` (via generated tools), and any external tools requiring Mangle.
//
// Deployment & Operations:
// - CI/CD: Standard Go build process, no special deployment.
//
// Code Quality Mandate:
// This shim must be minimal, simply re-exporting necessary components.
// It should not add significant logic or alter the behavior of the underlying `internal/mangle` package.
//
// Functions / Classes:
// - `NewEngine(config Config, contextLogger *zap.Logger)`: Re-exports `internal/mangle.NewEngine`.
// - `DefaultConfig()`: Re-exports `internal/mangle.DefaultConfig`.
// - `Config`: Re-exports `internal/mangle.Config`.
// - (and other necessary types/functions from internal/mangle)
//
// Usage:
// Tools that need to use the Mangle engine should import `codenerd/pkg/mangle` instead of `codenerd/internal/mangle`.
//
// --- END OF PRD HEADER ---

// Re-export core Mangle engine types and functions
type Engine = mangle.Engine
type Config = mangle.Config

var NewEngine = mangle.NewEngine
var DefaultConfig = mangle.DefaultConfig

// Re-export other necessary types
type Fact = ast.Atom
type Predicate = ast.Predicate
type PredicateSym = ast.PredicateSym
type Type = ast.Type
type ExternalFn = embed.ExternalFn
type Builtin = interpreter.Builtin
type Symbol = ast.Symbol
type UnificationError = ast.UnificationError
type Var = ast.Var

var Parse = parse.Parse
var ParseFile = parse.ParseFile
var ParseData = parse.ParseData
var DefaultSchema = schema.DefaultSchema
var NewSchema = schema.NewSchema
var NewSymbol = ast.NewSymbol
var IsRelationalSymbol = ast.IsRelationalSymbol
var NewPredicate = ast.NewPredicate
var SortAtoms = ast.SortAtoms
var DebuggerFromOptions = debug.DebuggerFromOptions
var NewGraph = topological.NewGraph
var Eval = semantics.Eval
var NewProfiler = profile.NewProfiler
var ConvertGoValueToAtom = mangle.ConvertGoValueToAtom
var FromGoValue = mangle.FromGoValue
var ToGoValue = mangle.ToGoValue
var IsExternalFact = mangle.IsExternalFact

// Placeholder for any context/logger needed by the re-exported functions
type ContextLogger = io.Writer
type Profiler = profile.Profiler
type Debugger = debug.Debugger

// Re-export types used in Config
type LogLevel = mangle.LogLevel
const (
	LogError = mangle.LogError
	LogWarn  = mangle.LogWarn
	LogInfo  = mangle.LogInfo
	LogDebug = mangle.LogDebug
)

// Re-export additional helper functions from mangle package if needed by tools.
var NewFact = mangle.NewFact
var DefaultMangleConfig = mangle.DefaultConfig
var NewMangleEngine = mangle.NewEngine
var NewMangleConfig = mangle.NewMangleConfig
var ConvertMangleType = mangle.ConvertMangleType
var IsPrimitiveType = mangle.IsPrimitiveType
var MustNewAtom = mangle.MustNewAtom

// Re-export any other common dependencies or types that tools might need
type Atom = ast.Atom
type Opts = mangle.Opts
type Graph = topological.Graph
type GoValueConverter = mangle.GoValueConverter
type GoValue = mangle.GoValue
type TypeCheckErr = schema.TypeCheckErr
type DeclarationError = schema.DeclarationError
type DuplicateDeclarationError = schema.DuplicateDeclarationError
type UnknownPredicateError = schema.UnknownPredicateError
type AmbiguousDeclarationError = schema.AmbiguousDeclarationError
type InterpreterError = interpreter.InterpreterError
type RuntimeError = interpreter.RuntimeError
type BuiltinFactory = interpreter.BuiltinFactory
type BuiltinMap = interpreter.BuiltinMap
type Options = interpreter.Options
type Node = topological.Node
type Edge = topological.Edge
type ProfileNode = profile.Node
type ProfileData = profile.Data
type ProfileOptions = profile.Options
type ProfileReport = profile.Report

// Additional functions or types used within check_mangle.go which need to be exposed.
type Fset = token.FileSet

var (
	MangleNewEngine          = mangle.NewEngine
	MangleDefaultConfig      = mangle.DefaultConfig
	MangleDefaultConfigWithOptions = mangle.DefaultConfigWithOptions
	MangleNewConfig          = mangle.NewMangleConfig
	MangleConvertGoValueToAtom = mangle.ConvertGoValueToAtom
	MangleToGoValue        = mangle.ToGoValue
	MangleFromGoValue      = mangle.FromGoValue
	MangleIsExternalFact   = mangle.IsExternalFact
)

type (
	MangleEngine = mangle.Engine
	MangleConfig = mangle.Config
	MangleOptions = mangle.Opts
	MangleFact = ast.Atom
	ManglePredicate = ast.Predicate
	ManglePredicateSym = ast.PredicateSym
	MangleType = ast.Type
	MangleExternalFn = embed.ExternalFn
	MangleBuiltin = interpreter.Builtin
	MangleSymbol = ast.Symbol
	MangleUnificationError = ast.UnificationError
	MangleVar = ast.Var
)

// Re-export everything from the parse package that might be needed by client code
var (
    ParseAtom = parse.ParseAtom
    ParseRule = parse.ParseRule
    ParseQuery = parse.ParseQuery
    ParseDecl = parse.ParseDecl
    ParseLiteral = parse.ParseLiteral
    ParseHead = parse.ParseHead
    ParseBody = parse.ParseBody
    ParseProgram = parse.ParseProgram
    ParseSchema = parse.ParseSchema
    ParseMap = parse.ParseMap
    ParseList = parse.ParseList
    ParseTerm = parse.ParseTerm
    ParseTermList = parse.ParseTermList
    ParseTransform = parse.ParseTransform
    ParseComparison = parse.ParseComparison
    ParseExpr = parse.ParseExpr
    ParseVar = parse.ParseVar
    ParseConst = parse.ParseConst
    ParseFunction = parse.ParseFunction
    ParseType = parse.ParseType
)

// Re-export everything from ast package
var (
    NewAtom          = ast.NewAtom
    NewName          = ast.NewName
    NewNumber        = ast.NewNumber
    NewString        = ast.NewString
    NewList          = ast.NewList
    NewMap           = ast.NewMap
    NewRule          = ast.NewRule
    NewDecl          = ast.NewDecl
    NewHead          = ast.NewHead
    NewBody          = ast.NewBody
    NewQuery         = ast.NewQuery
    NewLiteral       = ast.NewLiteral
    NewPredicateSymbol = ast.NewPredicateSym
    NewTypeExpr      = ast.NewTypeExpr
    NewTypeRef       = ast.NewTypeRef
    NewTypeUnion     = ast.NewTypeUnion
    NewTypeStruct    = ast.NewTypeStruct
    NewTypeArray     = ast.NewTypeArray
    NewTypeMap       = ast.NewTypeMap
    NewTypeFunction  = ast.NewTypeFunction
    NewTypeError     = ast.NewTypeError
    NewTransform     = ast.NewTransform
    NewComparison    = ast.NewComparison
    NewTerm          = ast.NewTerm
    NewVar           = ast.NewVar
    NewConst         = ast.NewConst
    NewFunction      = ast.NewFunction
    IsConstant       = ast.IsConstant
    IsVariable       = ast.IsVariable
    IsAtom           = ast.IsAtom
    IsRule           = ast.IsRule
    IsDecl           = ast.IsDecl
    IsPredicate      = ast.IsPredicate
    IsHead           = ast.IsHead
    IsBody           = ast.IsBody
    IsQuery          = ast.IsQuery
    IsLiteral        = ast.IsLiteral
    IsPredicateSymbol = ast.IsPredicateSym
    IsTypeExpr       = ast.IsTypeExpr
    IsTypeRef        = ast.IsTypeRef
    IsTypeUnion      = ast.IsTypeUnion
    IsTypeStruct     = ast.IsTypeStruct
    IsTypeArray      = ast.IsTypeArray
    IsTypeMap        = ast.IsTypeMap
    IsTypeFunction   = ast.IsTypeFunction
    IsTypeError      = ast.IsTypeError
    IsTransform      = ast.IsTransform
    IsComparison     = ast.IsComparison
    IsTerm           = ast.IsTerm
    IsVar            = ast.IsVar
    IsConst          = ast.IsConst
    IsFunction       = ast.IsFunction
    MapToTerms       = ast.MapToTerms
    MapFromTerms     = ast.MapFromTerms
    ListToTerms      = ast.ListToTerms
    ListFromTerms    = ast.ListFromTerms
    MapAtoms         = ast.MapAtoms
    SortTerms        = ast.SortTerms
    SortVarArray     = ast.SortVarArray
    Unify            = ast.Unify
    DeepCopyAtom     = ast.DeepCopyAtom
    DeepCopyRule     = ast.DeepCopyRule
    DeepCopyDecl     = ast.DeepCopyDecl
    DeepCopyQuery    = ast.DeepCopyQuery
    DeepCopyLiteral  = ast.DeepCopyLiteral
    DeepCopyTerm     = ast.DeepCopyTerm
    DeepCopyTransform = ast.DeepCopyTransform
    DeepCopyComparison = ast.DeepCopyComparison
    DeepCopyTypeExpr = ast.DeepCopyTypeExpr
    DeepCopyPredicateSym = ast.DeepCopyPredicateSym
    DeepCopyHead     = ast.DeepCopyHead
    DeepCopyBody     = ast.DeepCopyBody
)

// Re-export token package related types/functions
type (
    Pos = token.Pos
    Token = token.Token
    File = token.File
    FileSet = token.FileSet
)

var (
    NewFileSet = token.NewFileSet
)

// Re-export embed related
var (
    NewEmbedConfig = embed.NewConfig
    NewExternalFunctions = embed.NewExternalFunctions
    Register = embed.Register
    LoadDefaultBuiltins = embed.LoadDefaultBuiltins
)

// Re-export schema related
var (
    CheckDecl = schema.CheckDecl
    CheckProgram = schema.CheckProgram
    CheckQuery = schema.CheckQuery
    CheckTerms = schema.CheckTerms
    CheckAtom = schema.CheckAtom
    CheckLiteral = schema.CheckLiteral
    CheckRule = schema.CheckRule
    CheckMap = schema.CheckMap
    CheckList = schema.CheckList
    CheckExternalFunction = schema.CheckExternalFunction
    CheckTerm = schema.CheckTerm
    GetType = schema.GetType
    ApplySubstitution = schema.ApplySubstitution
    NewDeclarations = schema.NewDeclarations
    ParseAndCheckProgram = schema.ParseAndCheckProgram
)

// Re-export interpreter related
var (
    NewInterpreter = interpreter.NewInterpreter
    NewOptions = interpreter.NewOptions
    DefaultOptions = interpreter.DefaultOptions
    DefaultBuiltinMap = interpreter.DefaultBuiltinMap
    DefaultBuiltinFactory = interpreter.DefaultBuiltinFactory
    Run = interpreter.Run
    Query = interpreter.Query
    Load = interpreter.Load
    Process = interpreter.Process
    QueryInternal = interpreter.QueryInternal
    QueryExternal = interpreter.QueryExternal
    CollectBuiltins = interpreter.CollectBuiltins
)

// Re-export topological related
var (
    StronglyConnectedComponents = topological.StronglyConnectedComponents
    Tarjan = topological.Tarjan
    FindCycle = topological.FindCycle
    IsAcyclic = topological.IsAcyclic
    TopologicalSort = topological.TopologicalSort
    NewNode = topological.NewNode
    NewEdge = topological.NewEdge
)

// Re-export profile related
var (
    NewProfileReport = profile.NewReport
    NewProfileNode = profile.NewNode
    NewProfileData = profile.NewData
    StartProfiling = profile.StartProfiling
    StopProfiling = profile.StopProfiling
    ReportProfiling = profile.ReportProfiling
)

// Re-export debug related
var (
    NewDebugger = debug.NewDebugger
    DebugDefaultOptions = debug.DefaultOptions
    AddBreakpoint = debug.AddBreakpoint
    RemoveBreakpoint = debug.RemoveBreakpoint
    Continue = debug.Continue
    Step = debug.Step
    PrintStack = debug.PrintStack
    PrintVars = debug.PrintVars
)

// Re-export everything from the main `mangle` internal package that a tool might directly use
var (
	DefaultMangleEngineConfig = mangle.DefaultConfig
	NewMangleEngineWithConfig = mangle.NewEngine
	MangleLoadSchemaString    = mangle.Engine.LoadSchemaString
	MangleGetFacts            = mangle.Engine.GetFacts
)

// Ensure internal.mangle's context.Context and time.Duration are available
type Context = context.Context
type Duration = time.Duration