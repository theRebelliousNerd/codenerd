---
trigger: always_on
---

when merging branches on github, always investigate and reason over if the merge is a good idea. if it will make the codebase better, do it, if it is reverting code to legacy states or will cause other systems to break, or if its going to cause a wiring issue where its not fully integrated, make a plan to integrate. then make the merges, fix the problems, wire it all in, and be confident the system will improve.
