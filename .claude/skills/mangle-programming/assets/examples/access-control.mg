# Access Control Example
# Role-Based Access Control (RBAC) with hierarchical permissions
# Demonstrates negation, aggregation, and structured data
#
# Mangle v0.4.0 compatible

# =============================================================================
# Schema: Base Facts (EDB)
# =============================================================================

# Users in the system
Decl user(ID.Type<n>, Name.Type<string>, Active.Type<n>).

# Roles in the system
Decl role(ID.Type<n>, Name.Type<string>, Level.Type<int>).

# Role hierarchy (parent role grants child role permissions)
Decl role_inherits(Child.Type<n>, Parent.Type<n>).

# Direct role assignments
Decl user_role(User.Type<n>, Role.Type<n>).

# Resources in the system
Decl resource(ID.Type<n>, Type.Type<n>, Owner.Type<n>).

# Permission grants (role can perform action on resource type)
Decl permission(Role.Type<n>, Action.Type<n>, ResourceType.Type<n>).

# Explicit denials (override grants)
Decl deny(User.Type<n>, Action.Type<n>, Resource.Type<n>).

# =============================================================================
# Sample Data
# =============================================================================

# Users
user(/user/alice, "Alice Admin", /active).
user(/user/bob, "Bob Developer", /active).
user(/user/charlie, "Charlie Viewer", /active).
user(/user/dave, "Dave Suspended", /suspended).

# Roles with privilege levels
role(/role/admin, "Administrator", 100).
role(/role/developer, "Developer", 50).
role(/role/viewer, "Viewer", 10).
role(/role/guest, "Guest", 0).

# Role hierarchy
role_inherits(/role/developer, /role/viewer).
role_inherits(/role/admin, /role/developer).

# User-role assignments
user_role(/user/alice, /role/admin).
user_role(/user/bob, /role/developer).
user_role(/user/charlie, /role/viewer).
user_role(/user/dave, /role/developer).

# Resources
resource(/res/database, /type/data, /user/alice).
resource(/res/api, /type/service, /user/bob).
resource(/res/docs, /type/document, /user/charlie).
resource(/res/config, /type/config, /user/alice).

# Permissions
permission(/role/admin, /action/read, /type/data).
permission(/role/admin, /action/write, /type/data).
permission(/role/admin, /action/delete, /type/data).
permission(/role/admin, /action/admin, /type/config).

permission(/role/developer, /action/read, /type/data).
permission(/role/developer, /action/write, /type/data).
permission(/role/developer, /action/deploy, /type/service).

permission(/role/viewer, /action/read, /type/data).
permission(/role/viewer, /action/read, /type/document).

# Explicit denials
deny(/user/bob, /action/delete, /res/database).

# =============================================================================
# Rules: Permission Resolution (IDB)
# =============================================================================

# Effective roles (including inherited)
effective_role(User, Role) :-
    user_role(User, Role).

effective_role(User, ParentRole) :-
    user_role(User, ChildRole),
    role_inherits(ChildRole, ParentRole).

# Transitive role inheritance
role_ancestor(Child, Ancestor) :-
    role_inherits(Child, Ancestor).

role_ancestor(Child, Ancestor) :-
    role_inherits(Child, Parent),
    role_ancestor(Parent, Ancestor).

# Permission through any effective role
has_permission_base(User, Action, ResourceType) :-
    effective_role(User, Role),
    permission(Role, Action, ResourceType).

# Check if user can perform action on specific resource
can_access(User, Action, Resource) :-
    user(User, _, /active),
    resource(Resource, ResourceType, _),
    has_permission_base(User, Action, ResourceType),
    !deny(User, Action, Resource).

# Cannot access (denied or no permission)
cannot_access(User, Action, Resource) :-
    user(User, _, /active),
    resource(Resource, _, _),
    deny(User, Action, Resource).

cannot_access(User, Action, Resource) :-
    user(User, _, /active),
    resource(Resource, ResourceType, _),
    !has_permission_base(User, Action, ResourceType).

# Suspended users cannot access anything
cannot_access(User, Action, Resource) :-
    user(User, _, /suspended),
    resource(Resource, _, _).

# Resource owners always have full access
can_access(User, Action, Resource) :-
    user(User, _, /active),
    resource(Resource, _, User).

# =============================================================================
# Rules: Analysis and Reporting (IDB)
# =============================================================================

# Find users with admin privileges
admin_users(User, Name) :-
    user(User, Name, /active),
    effective_role(User, /role/admin).

# Find over-privileged users (high role level but few resources)
user_privilege_level(User, MaxLevel) :-
    effective_role(User, Role),
    role(Role, _, Level) |>
    do fn:group_by(User),
    let MaxLevel = fn:max(Level).

# Count permissions per user
permission_count(User, Count) :-
    has_permission_base(User, _, _) |>
    do fn:group_by(User),
    let Count = fn:count().

# Find resources with no access controls
unprotected_resource(Resource) :-
    resource(Resource, Type, _),
    !permission(_, _, Type).

# Audit: who can access what
access_audit(User, UserName, Resource, Action) :-
    user(User, UserName, _),
    resource(Resource, _, _),
    can_access(User, Action, Resource).

# =============================================================================
# Queries (for REPL)
# =============================================================================

# ?can_access(/user/alice, /action/delete, /res/database)
# ?can_access(/user/bob, /action/delete, /res/database)
# ?cannot_access(User, Action, Resource)
# ?admin_users(User, Name)
# ?access_audit(User, Name, /res/database, Action)
# ?permission_count(User, Count)
