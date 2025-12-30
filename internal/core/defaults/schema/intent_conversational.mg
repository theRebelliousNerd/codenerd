# Intent Definitions - Conversational Interactions
# SECTIONS 2-3: CAPABILITIES, HELP & GREETINGS - DIRECT RESPONSE, NO SHARD
# Social interactions and help requests that don't require code actions.

# =============================================================================
# SECTION 2: CAPABILITIES & HELP (/help)
# Questions about what the agent can do - answered directly.
# =============================================================================

intent_definition("What can you do?", /help, "capabilities").
intent_category("What can you do?", /query).

intent_definition("What are your capabilities?", /help, "capabilities").
intent_category("What are your capabilities?", /query).

intent_definition("Help.", /help, "general").
intent_category("Help.", /query).

intent_definition("Help me.", /help, "general").
intent_category("Help me.", /query).

intent_definition("What commands are available?", /help, "commands").
intent_category("What commands are available?", /query).

intent_definition("List commands.", /help, "commands").
intent_category("List commands.", /query).

intent_definition("Show me what you can do.", /help, "capabilities").
intent_category("Show me what you can do.", /query).

intent_definition("What features do you have?", /help, "capabilities").
intent_category("What features do you have?", /query).

intent_definition("How do I use you?", /help, "usage").
intent_category("How do I use you?", /query).

intent_definition("Getting started.", /help, "usage").
intent_category("Getting started.", /query).

intent_definition("Tutorial.", /help, "usage").
intent_category("Tutorial.", /query).

intent_definition("How does this work?", /help, "usage").
intent_category("How does this work?", /query).

intent_definition("Can you review code?", /help, "capabilities").
intent_category("Can you review code?", /query).

intent_definition("Can you write code?", /help, "capabilities").
intent_category("Can you write code?", /query).

intent_definition("Can you run tests?", /help, "capabilities").
intent_category("Can you run tests?", /query).

intent_definition("Can you search files?", /help, "capabilities").
intent_category("Can you search files?", /query).

intent_definition("Do you have access to the file system?", /help, "capabilities").
intent_category("Do you have access to the file system?", /query).

intent_definition("Can you execute commands?", /help, "capabilities").
intent_category("Can you execute commands?", /query).

intent_definition("What shards do you have?", /help, "shards").
intent_category("What shards do you have?", /query).

intent_definition("What agents are available?", /help, "shards").
intent_category("What agents are available?", /query).

intent_definition("List available agents.", /help, "shards").
intent_category("List available agents.", /query).

intent_definition("What specialists exist?", /help, "shards").
intent_category("What specialists exist?", /query).

intent_definition("How do I start a campaign?", /help, "campaign").
intent_category("How do I start a campaign?", /query).

intent_definition("What is a campaign?", /help, "campaign").
intent_category("What is a campaign?", /query).

intent_definition("How do I define a new agent?", /help, "define_agent").
intent_category("How do I define a new agent?", /query).

intent_definition("How do I create a specialist?", /help, "define_agent").
intent_category("How do I create a specialist?", /query).

intent_definition("What is autopoiesis?", /help, "autopoiesis").
intent_category("What is autopoiesis?", /query).

intent_definition("Can you learn?", /help, "learning").
intent_category("Can you learn?", /query).

intent_definition("Do you remember things?", /help, "memory").
intent_category("Do you remember things?", /query).

intent_definition("What is Mangle?", /help, "mangle").
intent_category("What is Mangle?", /query).

intent_definition("What is the kernel?", /help, "kernel").
intent_category("What is the kernel?", /query).

intent_definition("Explain your architecture.", /help, "architecture").
intent_category("Explain your architecture.", /query).

intent_definition("How are you built?", /help, "architecture").
intent_category("How are you built?", /query).

# =============================================================================
# SECTION 3: GREETINGS & CONVERSATION (/greet)
# Social interactions that don't require any code actions.
# =============================================================================

intent_definition("Hello.", /greet, "hello").
intent_category("Hello.", /query).

intent_definition("Hi.", /greet, "hello").
intent_category("Hi.", /query).

intent_definition("Hi there.", /greet, "hello").
intent_category("Hi there.", /query).

intent_definition("Hey.", /greet, "hello").
intent_category("Hey.", /query).

intent_definition("Hey there.", /greet, "hello").
intent_category("Hey there.", /query).

intent_definition("Good morning.", /greet, "hello").
intent_category("Good morning.", /query).

intent_definition("Good afternoon.", /greet, "hello").
intent_category("Good afternoon.", /query).

intent_definition("Good evening.", /greet, "hello").
intent_category("Good evening.", /query).

intent_definition("Howdy.", /greet, "hello").
intent_category("Howdy.", /query).

intent_definition("What's up?", /greet, "hello").
intent_category("What's up?", /query).

intent_definition("Sup.", /greet, "hello").
intent_category("Sup.", /query).

intent_definition("Yo.", /greet, "hello").
intent_category("Yo.", /query).

intent_definition("Thanks!", /greet, "thanks").
intent_category("Thanks!", /query).

intent_definition("Thank you.", /greet, "thanks").
intent_category("Thank you.", /query).

intent_definition("Thanks a lot.", /greet, "thanks").
intent_category("Thanks a lot.", /query).

intent_definition("Much appreciated.", /greet, "thanks").
intent_category("Much appreciated.", /query).

intent_definition("Cheers.", /greet, "thanks").
intent_category("Cheers.", /query).

intent_definition("Awesome, thanks.", /greet, "thanks").
intent_category("Awesome, thanks.", /query).

intent_definition("Perfect, thank you.", /greet, "thanks").
intent_category("Perfect, thank you.", /query).

intent_definition("Goodbye.", /greet, "bye").
intent_category("Goodbye.", /query).

intent_definition("Bye.", /greet, "bye").
intent_category("Bye.", /query).

intent_definition("See you.", /greet, "bye").
intent_category("See you.", /query).

intent_definition("Later.", /greet, "bye").
intent_category("Later.", /query).

intent_definition("Good work.", /greet, "praise").
intent_category("Good work.", /query).

intent_definition("Nice job.", /greet, "praise").
intent_category("Nice job.", /query).

intent_definition("Well done.", /greet, "praise").
intent_category("Well done.", /query).

intent_definition("That's great.", /greet, "praise").
intent_category("That's great.", /query).

intent_definition("Okay.", /greet, "ack").
intent_category("Okay.", /query).

intent_definition("OK.", /greet, "ack").
intent_category("OK.", /query).

intent_definition("Got it.", /greet, "ack").
intent_category("Got it.", /query).

intent_definition("I see.", /greet, "ack").
intent_category("I see.", /query).

intent_definition("Makes sense.", /greet, "ack").
intent_category("Makes sense.", /query).

intent_definition("Understood.", /greet, "ack").
intent_category("Understood.", /query).
