#!/usr/bin/env python3
import sys

def read_file(filename):
    with open(filename, 'r', encoding='utf-8') as f:
        return f.readlines()

def extract_sections_by_category(lines):
    result = {
        'query': [],
        'mutation': [],
        'instruction': [],
        'campaign': [],
        'multistep_patterns': [],
        'multistep_corpus': []
    }
    
    current_section = None
    current_section_lines = []
    section_category = None
    
    i = 0
    while i < len(lines):
        line = lines[i]
        
        if i < 14:
            i += 1
            continue
        
        if line.startswith('# ====') and i + 1 < len(lines) and lines[i+1].startswith('# SECTION'):
            if current_section and current_section_lines and section_category:
                result[section_category].append((current_section, current_section_lines))
            
            current_section = lines[i+1].rstrip()
            current_section_lines = [line, lines[i+1]]
            
            if 'CAMPAIGN' in current_section:
                section_category = 'campaign'
            elif 'MULTI-STEP TASK PATTERNS' in current_section:
                section_category = 'multistep_patterns'
            elif 'ENCYCLOPEDIC MULTI-STEP' in current_section:
                section_category = 'multistep_corpus'
            elif 'CONFIGURATION' in current_section:
                section_category = 'instruction'
            else:
                query_count = sum(1 for l in lines[i:min(i+100, len(lines))] if '/query' in l)
                mutation_count = sum(1 for l in lines[i:min(i+100, len(lines))] if '/mutation' in l)
                instruction_count = sum(1 for l in lines[i:min(i+100, len(lines))] if '/instruction' in l)
                
                if instruction_count > 0:
                    section_category = 'instruction'
                elif mutation_count > query_count:
                    section_category = 'mutation'
                else:
                    section_category = 'query'
            
            i += 2
            continue
        
        if current_section:
            current_section_lines.append(line)
        
        i += 1
    
    if current_section and current_section_lines and section_category:
        result[section_category].append((current_section, current_section_lines))
    
    return result

lines = read_file('intent.mg')
sections = extract_sections_by_category(lines)

# Core
with open('intent_core.mg', 'w', encoding='utf-8') as f:
    f.write("# Intent Core - Decl statements\n\n")
    for i in range(11, 14):
        if i < len(lines) and lines[i].strip():
            f.write(lines[i])
    f.write('\n')
    for i, line in enumerate(lines):
        if i >= 1820 and i <= 1830 and 'Decl multistep' in line:
            f.write(line)
        elif i >= 2370 and i <= 2400 and 'Decl ' in line:
            f.write(line)

# Queries
with open('intent_queries.mg', 'w') as f:
    f.write("# Intent Queries\n\n")
    for _, sec_lines in sections['query']:
        f.write(''.join(sec_lines))
        f.write('\n')

# Mutations
with open('intent_mutations.mg', 'w') as f:
    f.write("# Intent Mutations\n\n")
    for _, sec_lines in sections['mutation']:
        f.write(''.join(sec_lines))
        f.write('\n')

# Instructions
with open('intent_instructions.mg', 'w') as f:
    f.write("# Intent Instructions\n\n")
    for _, sec_lines in sections['instruction']:
        f.write(''.join(sec_lines))
        f.write('\n')

# Campaign
with open('intent_campaign.mg', 'w') as f:
    f.write("# Intent Campaign\n\n")
    all_campaign = sections['campaign'] + sections['multistep_patterns'] + sections['multistep_corpus']
    for _, sec_lines in all_campaign:
        f.write(''.join(sec_lines))
        f.write('\n')

# System
with open('intent_system.mg', 'w') as f:
    f.write("""# Intent System - Inference Rules

pattern_verb_pair(Pattern, Verb1, Verb2) :-
    multistep_verb_pair(Pattern, Verb1, Verb2).

pattern_relation(Pattern, Relation) :-
    multistep_pattern(Pattern, _, Relation, _).
""")

print("Created all 6 modular files")
