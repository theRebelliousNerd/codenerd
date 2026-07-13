# TODO — shards architecture backlog

> Last verified against codebase: 2026-07-13  
> Prioritized; docs-only corpus does not implement these

## P0

- [ ] Verify boot-guard disable sites (chat + all CLI paths) never fire before first intentional user/command turn  
- [ ] Confirm mangle_repair interceptor wired on every boot that loads learned rules  

## P1

- [ ] Unify `session_boot.go` factories with `RegisterAllShardFactories` + RegistryContext extensions / post-spawn hooks  
- [ ] Consume `DefaultShardPredicateManifests` in `internal/system/factory.go` KernelShard construction  
- [ ] Add registration-set equality test (factory names vs chat names)  
- [ ] Ensure campaign_runner always receives `SetShardManager` on all boots  

## P2

- [ ] Refresh `internal/shards/README.md` to match live system-shard reality (or point to this corpus)  
- [ ] Embedding-assisted specialist matching (retrieve → structured match)  
- [ ] Expand full OODA integration tests in-package (boot guard, deny path, permit→route)  
- [ ] Align legislator profile ModelConfig with constructor  

## P3

- [ ] GlassBox/ToolEventBus/ToolStore parity on factory boot path  
- [ ] Clean historical `*_learnings.db` artifacts if unused  
- [ ] Central metrics export for CostGuard / executive counters  
- [ ] Document action_linter vs router route table as CI check  

## Done (reference)

- Domain Go shards removed; JIT personas + session executor  
- Event-driven executive/constitution/router with poll fallback  
- CostGuard + autopoiesis WaitGroup on executive  
- Predicate manifest data structure exported  
