# Aggregation Patterns Example
# Comprehensive examples of Mangle aggregation and transforms
# Demonstrates all aggregation functions and patterns
#
# Mangle v0.4.0 compatible

# =============================================================================
# Schema: Sample Data
# =============================================================================

Decl sale(Region.Type<n>, Product.Type<n>, Amount.Type<float>, Quantity.Type<int>).
Decl employee(ID.Type<n>, Name.Type<string>, Department.Type<n>, Salary.Type<float>).
Decl project_assignment(EmployeeID.Type<n>, ProjectID.Type<n>, Hours.Type<int>).
Decl log_entry(Timestamp.Type<int>, Level.Type<n>, Message.Type<string>).

# =============================================================================
# Sample Data
# =============================================================================

# Sales data
sale(/us_east, /widget_a, 100.00, 10).
sale(/us_east, /widget_a, 150.00, 15).
sale(/us_east, /widget_b, 200.00, 20).
sale(/us_west, /widget_a, 120.00, 12).
sale(/us_west, /widget_b, 180.00, 18).
sale(/eu, /widget_a, 90.00, 9).
sale(/eu, /widget_b, 110.00, 11).
sale(/eu, /widget_c, 75.00, 5).

# Employee data
employee(/emp/1, "Alice", /engineering, 120000.0).
employee(/emp/2, "Bob", /engineering, 110000.0).
employee(/emp/3, "Charlie", /sales, 95000.0).
employee(/emp/4, "Diana", /sales, 105000.0).
employee(/emp/5, "Eve", /engineering, 130000.0).
employee(/emp/6, "Frank", /hr, 85000.0).

# Project assignments
project_assignment(/emp/1, /proj/alpha, 20).
project_assignment(/emp/1, /proj/beta, 20).
project_assignment(/emp/2, /proj/alpha, 40).
project_assignment(/emp/3, /proj/gamma, 30).
project_assignment(/emp/4, /proj/gamma, 25).
project_assignment(/emp/5, /proj/beta, 40).

# Log entries
log_entry(1000, /error, "Connection failed").
log_entry(1001, /warning, "Slow query").
log_entry(1002, /error, "Timeout").
log_entry(1003, /info, "Request completed").
log_entry(1004, /error, "Connection failed").
log_entry(1005, /warning, "Memory high").

# =============================================================================
# PATTERN 1: Simple Count
# =============================================================================

# Count all sales
total_sales_count(N) :-
    sale(_, _, _, _) |>
    do fn:group_by(),
    let N = fn:count().

# Count by region
sales_count_by_region(Region, N) :-
    sale(Region, _, _, _) |>
    do fn:group_by(Region),
    let N = fn:count().

# =============================================================================
# PATTERN 2: Sum Aggregation
# =============================================================================

# Total revenue
total_revenue(Total) :-
    sale(_, _, Amount, _) |>
    do fn:group_by(),
    let Total = fn:sum(Amount).

# Revenue by region
revenue_by_region(Region, Total) :-
    sale(Region, _, Amount, _) |>
    do fn:group_by(Region),
    let Total = fn:sum(Amount).

# Revenue by product
revenue_by_product(Product, Total) :-
    sale(_, Product, Amount, _) |>
    do fn:group_by(Product),
    let Total = fn:sum(Amount).

# =============================================================================
# PATTERN 3: Min/Max
# =============================================================================

# Highest sale amount
max_sale_amount(Max) :-
    sale(_, _, Amount, _) |>
    do fn:group_by(),
    let Max = fn:max(Amount).

# Lowest sale amount by region
min_sale_by_region(Region, Min) :-
    sale(Region, _, Amount, _) |>
    do fn:group_by(Region),
    let Min = fn:min(Amount).

# Salary range by department
salary_range(Dept, MinSal, MaxSal) :-
    employee(_, _, Dept, Salary) |>
    do fn:group_by(Dept),
    let MinSal = fn:min(Salary),
    let MaxSal = fn:max(Salary).

# =============================================================================
# PATTERN 4: Multi-Variable Grouping
# =============================================================================

# Sales by region AND product
sales_by_region_product(Region, Product, Total, Count) :-
    sale(Region, Product, Amount, _) |>
    do fn:group_by(Region, Product),
    let Total = fn:sum(Amount),
    let Count = fn:count().

# =============================================================================
# PATTERN 5: Average (Computed from Sum and Count)
# =============================================================================

# Average sale amount
average_sale(Avg) :-
    sale(_, _, Amount, _) |>
    do fn:group_by(),
    let Total = fn:sum(Amount),
    let Count = fn:count() |>
    let Avg = fn:divide(Total, Count).

# Average salary by department
avg_salary_by_dept(Dept, Avg) :-
    employee(_, _, Dept, Salary) |>
    do fn:group_by(Dept),
    let Total = fn:sum(Salary),
    let Count = fn:count() |>
    let Avg = fn:divide(Total, Count).

# =============================================================================
# PATTERN 6: Filtering Before Aggregation
# =============================================================================

# Count only error logs
error_count(N) :-
    log_entry(_, /error, _) |>
    do fn:group_by(),
    let N = fn:count().

# Revenue from widget_a only
widget_a_revenue(Total) :-
    sale(_, /widget_a, Amount, _) |>
    do fn:group_by(),
    let Total = fn:sum(Amount).

# High earners count by department
high_earner_count(Dept, N) :-
    employee(_, _, Dept, Salary),
    Salary > 100000.0 |>
    do fn:group_by(Dept),
    let N = fn:count().

# =============================================================================
# PATTERN 7: Aggregation with Join
# =============================================================================

# Total hours per employee (with name)
employee_total_hours(Name, TotalHours) :-
    employee(EmpID, Name, _, _),
    project_assignment(EmpID, _, Hours) |>
    do fn:group_by(Name),
    let TotalHours = fn:sum(Hours).

# Project hours by department
project_hours_by_dept(Dept, ProjectID, TotalHours) :-
    employee(EmpID, _, Dept, _),
    project_assignment(EmpID, ProjectID, Hours) |>
    do fn:group_by(Dept, ProjectID),
    let TotalHours = fn:sum(Hours).

# =============================================================================
# PATTERN 8: Nested Aggregation (Two-Stage)
# =============================================================================

# First: count per region
# Then: average count across regions
# (Requires intermediate predicate)

region_sale_count(Region, Count) :-
    sale(Region, _, _, _) |>
    do fn:group_by(Region),
    let Count = fn:count().

avg_sales_per_region(Avg) :-
    region_sale_count(_, Count) |>
    do fn:group_by(),
    let Total = fn:sum(Count),
    let Num = fn:count() |>
    let Avg = fn:divide(Total, Num).

# =============================================================================
# PATTERN 9: Conditional Aggregation
# =============================================================================

# Count sales above threshold per region
large_sale_count(Region, Count) :-
    sale(Region, _, Amount, _),
    Amount > 100.0 |>
    do fn:group_by(Region),
    let Count = fn:count().

# Ratio of large sales to total (requires two aggregations)
region_sale_totals(Region, Total) :-
    sale(Region, _, _, _) |>
    do fn:group_by(Region),
    let Total = fn:count().

large_sale_ratio(Region, Ratio) :-
    large_sale_count(Region, Large),
    region_sale_totals(Region, Total) |>
    let Ratio = fn:divide(Large, Total).

# =============================================================================
# PATTERN 10: Existence Checks with Aggregation
# =============================================================================

# Departments with at least 2 employees
large_department(Dept) :-
    employee(_, _, Dept, _) |>
    do fn:group_by(Dept),
    let Count = fn:count(),
    Count >= 2.

# Projects with multiple assignees
shared_project(ProjectID) :-
    project_assignment(_, ProjectID, _) |>
    do fn:group_by(ProjectID),
    let Count = fn:count(),
    Count > 1.

# =============================================================================
# Queries (for REPL)
# =============================================================================

# ?total_sales_count(N)
# ?sales_count_by_region(Region, N)
# ?revenue_by_region(Region, Total)
# ?sales_by_region_product(Region, Product, Total, Count)
# ?average_sale(Avg)
# ?avg_salary_by_dept(Dept, Avg)
# ?employee_total_hours(Name, Hours)
# ?large_department(Dept)
