#!/usr/bin/env python3
import sqlite3
import os
import sys

def check_knowledge_db(db_path='.nerd/knowledge.db'):
    """Check the status and contents of the knowledge database."""
    if not os.path.exists(db_path):
        print(f"ERROR: {db_path} does not exist!", file=sys.stderr)
        return 1

    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        # Get all tables
        tables = cursor.execute("SELECT name FROM sqlite_master WHERE type='table';").fetchall()
        table_names = [t[0] for t in tables]

        print(f"=== Knowledge Database Status ===")
        print(f"Location: {db_path}")
        print(f"Tables: {len(table_names)}")
        print()

        total_rows = 0

        # Count rows in each table
        for table in table_names:
            try:
                count = cursor.execute(f'SELECT COUNT(*) FROM {table}').fetchone()[0]
                total_rows += count
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
        print(f"Total rows across all tables: {total_rows}")

        conn.close()
        return 0

    except sqlite3.Error as e:
        print(f"Database error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Unexpected error: {e}", file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(check_knowledge_db())