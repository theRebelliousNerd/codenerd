#!/usr/bin/env python3
import sqlite3
import os

db_path = '.nerd/knowledge.db'

if not os.path.exists(db_path):
    print(f"ERROR: {db_path} does not exist!")
    exit(1)

conn = sqlite3.connect(db_path)
cursor = conn.cursor()

# Get all tables
tables = cursor.execute("SELECT name FROM sqlite_master WHERE type='table';").fetchall()
table_names = [t[0] for t in tables]

print(f"=== Knowledge Database Status ===")
print(f"Location: {db_path}")
print(f"Tables: {len(table_names)}")
print()

# Count rows in each table
for table in table_names:
    try:
        count = cursor.execute(f'SELECT COUNT(*) FROM {table}').fetchone()[0]
        print(f"  {table:25} {count:6} rows")

        # Show sample data for non-empty tables
        if count > 0 and count <= 3:
            print(f"    Sample:")
            rows = cursor.execute(f'SELECT * FROM {table} LIMIT 3').fetchall()
            for row in rows:
                print(f"      {row}")
    except Exception as e:
        print(f"  {table:25} ERROR: {e}")

print()
print(f"=== Summary ===")
total_rows = sum([cursor.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0] for t in table_names])
print(f"Total rows across all tables: {total_rows}")

conn.close()
