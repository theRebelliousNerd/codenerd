# User Policy Overrides
# Define project-specific rules here.
# These can extend or override core behavior.

# Example: Allow deleting .tmp files even if modified
# permitted(Action) :- 
#     action_type(Action, /delete_file),
#     target_path(Action, Path),
#     fn:string_suffix(Path, ".tmp").
