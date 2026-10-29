# JIT Prompt Compiler Configuration
# Extracted from jit_compiler.mg
# Defines static ordering and budget allocation for prompt categories.

# Category Ordering (Static Facts)
# Determines section order in final prompt.
# Lower numbers appear first in the assembled prompt.

category_order(/identity, 1).
category_order(/safety, 2).
category_order(/hallucination, 3).
category_order(/methodology, 4).
category_order(/language, 5).
category_order(/framework, 6).
category_order(/domain, 7).
category_order(/campaign, 8).
category_order(/init, 8).
category_order(/northstar, 8).
category_order(/ouroboros, 8).
category_order(/context, 9).
category_order(/exemplar, 10).
category_order(/protocol, 11).

# Category Budget Allocation
# Percentage of total token budget allocated to each category.

category_budget(/identity, 5).
category_budget(/protocol, 12).
category_budget(/safety, 5).
category_budget(/hallucination, 8).
category_budget(/methodology, 15).
category_budget(/language, 8).
category_budget(/framework, 8).
category_budget(/domain, 15).
category_budget(/context, 12).
category_budget(/exemplar, 7).
category_budget(/campaign, 5).
category_budget(/init, 5).
category_budget(/northstar, 5).
category_budget(/ouroboros, 5).
