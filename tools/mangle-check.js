#!/usr/bin/env node
/**
 * Batch Mangle file checker using the Mangle LSP parser and validators.
 *
 * Usage:
 *   node tools/mangle-check.js [--errors-only] [pattern]
 *
 * Examples:
 *   node tools/mangle-check.js                    # Check all .mg files
 *   node tools/mangle-check.js --errors-only      # Only show errors (no warnings)
 *   node tools/mangle-check.js internal/core      # Check files in specific dir
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

// Path to LSP extension - auto-detect version
function findMangleLspPath() {
    const extensionsDir = path.join(os.homedir(), '.vscode', 'extensions');
    const entries = fs.readdirSync(extensionsDir);
    const mangleExt = entries.find(e => e.startsWith('mangle.mangle-vscode-'));
    if (!mangleExt) {
        throw new Error('Mangle VSCode extension not found. Install it from the marketplace.');
    }
    return path.join(extensionsDir, mangleExt, 'server');
}
const LSP_PATH = findMangleLspPath();

// Import parser and analysis from the LSP
const { parse, hasErrors, hasFatalErrors, getErrorCounts } = require(path.join(LSP_PATH, 'parser', 'index.js'));
const { validate, checkStratification, checkUnboundedRecursion, checkCartesianExplosion, checkLateFiltering, checkLateNegation, checkMultipleIndependentVars } = require(path.join(LSP_PATH, 'analysis', 'index.js'));

// ANSI colors
const colors = {
    reset: '\x1b[0m',
    red: '\x1b[31m',
    yellow: '\x1b[33m',
    green: '\x1b[32m',
    cyan: '\x1b[36m',
    gray: '\x1b[90m',
    bold: '\x1b[1m',
};

/**
 * Find all .mg files recursively.
 */
function findMgFiles(dir, pattern = null) {
    const results = [];

    function walk(currentDir) {
        try {
            const entries = fs.readdirSync(currentDir, { withFileTypes: true });
            for (const entry of entries) {
                const fullPath = path.join(currentDir, entry.name);
                if (entry.isDirectory()) {
                    // Skip node_modules, .git, vendor
                    if (!['node_modules', '.git', 'vendor'].includes(entry.name)) {
                        walk(fullPath);
                    }
                } else if (entry.isFile() && entry.name.endsWith('.mg')) {
                    if (!pattern || fullPath.includes(pattern)) {
                        results.push(fullPath);
                    }
                }
            }
        } catch (e) {
            // Skip unreadable directories
        }
    }

    walk(dir);
    return results.sort();
}

/**
 * Check a single .mg file.
 */
function checkFile(filePath) {
    const content = fs.readFileSync(filePath, 'utf-8');
    const issues = [];

    // Parse
    const parseResult = parse(content);

    // Collect parse errors
    for (const error of parseResult.errors) {
        issues.push({
            type: 'error',
            source: error.source === 'lexer' ? 'lexer' : 'parser',
            line: error.line,
            column: error.column,
            message: error.message,
        });
    }

    // Run semantic validation if parse succeeded
    if (parseResult.unit) {
        const validationResult = validate(parseResult.unit);

        for (const error of validationResult.errors) {
            issues.push({
                type: error.severity || 'error',
                source: 'semantic',
                line: error.range.start.line,
                column: error.range.start.column,
                message: error.message,
                code: error.code,
            });
        }

        // Stratification errors
        const stratErrors = checkStratification(parseResult.unit);
        for (const error of stratErrors) {
            issues.push({
                type: error.severity || 'error',
                source: 'stratification',
                line: error.range.start.line,
                column: error.range.start.column,
                message: error.message,
                code: error.code,
            });
        }

        // Unbounded recursion
        const recursionWarnings = checkUnboundedRecursion(parseResult.unit);
        for (const warning of recursionWarnings) {
            issues.push({
                type: warning.severity || 'warning',
                source: 'recursion',
                line: warning.range.start.line,
                column: warning.range.start.column,
                message: warning.message,
                code: warning.code,
            });
        }

        // Cartesian explosion
        const cartesianWarnings = checkCartesianExplosion(parseResult.unit);
        for (const warning of cartesianWarnings) {
            issues.push({
                type: warning.severity || 'warning',
                source: 'cartesian',
                line: warning.range.start.line,
                column: warning.range.start.column,
                message: warning.message,
                code: warning.code,
            });
        }

        // Late filtering
        const lateFilterWarnings = checkLateFiltering(parseResult.unit);
        for (const warning of lateFilterWarnings) {
            issues.push({
                type: warning.severity || 'warning',
                source: 'late-filter',
                line: warning.range.start.line,
                column: warning.range.start.column,
                message: warning.message,
                code: warning.code,
            });
        }

        // Late negation
        const lateNegationWarnings = checkLateNegation(parseResult.unit);
        for (const warning of lateNegationWarnings) {
            issues.push({
                type: warning.severity || 'warning',
                source: 'late-negation',
                line: warning.range.start.line,
                column: warning.range.start.column,
                message: warning.message,
                code: warning.code,
            });
        }

        // Multiple independent variables
        const multiIndepWarnings = checkMultipleIndependentVars(parseResult.unit);
        for (const warning of multiIndepWarnings) {
            issues.push({
                type: warning.severity || 'warning',
                source: 'multi-indep',
                line: warning.range.start.line,
                column: warning.range.start.column,
                message: warning.message,
                code: warning.code,
            });
        }
    }

    return issues;
}

/**
 * Format issue for display.
 */
function formatIssue(filePath, issue, rootDir) {
    const relPath = path.relative(rootDir, filePath);
    const typeColor = issue.type === 'error' ? colors.red : colors.yellow;
    const typeLabel = issue.type === 'error' ? 'error' : 'warn ';

    return `${colors.cyan}${relPath}${colors.reset}:${issue.line}:${issue.column} ${typeColor}${typeLabel}${colors.reset} [${colors.gray}${issue.source}${colors.reset}] ${issue.message}`;
}

/**
 * Main entry point.
 */
function main() {
    const args = process.argv.slice(2);
    const errorsOnly = args.includes('--errors-only');
    const pattern = args.find(a => !a.startsWith('--'));

    const rootDir = process.cwd();
    console.log(`${colors.bold}Mangle File Checker${colors.reset}`);
    console.log(`${colors.gray}Using LSP from: ${LSP_PATH}${colors.reset}\n`);

    // Find files
    const files = findMgFiles(rootDir, pattern);
    console.log(`Found ${files.length} .mg files${pattern ? ` matching '${pattern}'` : ''}\n`);

    if (files.length === 0) {
        console.log('No files to check.');
        return;
    }

    let totalErrors = 0;
    let totalWarnings = 0;
    let filesWithIssues = 0;

    for (const file of files) {
        try {
            const issues = checkFile(file);
            const errors = issues.filter(i => i.type === 'error');
            const warnings = issues.filter(i => i.type !== 'error');

            const displayIssues = errorsOnly ? errors : issues;

            if (displayIssues.length > 0) {
                filesWithIssues++;
                for (const issue of displayIssues) {
                    console.log(formatIssue(file, issue, rootDir));
                }
            }

            totalErrors += errors.length;
            totalWarnings += warnings.length;
        } catch (e) {
            console.log(`${colors.red}Error checking ${path.relative(rootDir, file)}: ${e.message}${colors.reset}`);
            totalErrors++;
        }
    }

    // Summary
    console.log();
    if (totalErrors === 0 && totalWarnings === 0) {
        console.log(`${colors.green}✓ All ${files.length} files passed!${colors.reset}`);
    } else {
        console.log(`${colors.bold}Summary:${colors.reset} ${files.length} files checked, ${filesWithIssues} with issues`);
        if (totalErrors > 0) {
            console.log(`  ${colors.red}${totalErrors} error(s)${colors.reset}`);
        }
        if (totalWarnings > 0 && !errorsOnly) {
            console.log(`  ${colors.yellow}${totalWarnings} warning(s)${colors.reset}`);
        }
    }

    // Exit with error code if there are errors
    process.exit(totalErrors > 0 ? 1 : 0);
}

main();
