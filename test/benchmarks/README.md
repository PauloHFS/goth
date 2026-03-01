# Benchmark Performance Suite

Este diretório contém a suíte completa de benchmarks para o projeto GOTH, incluindo:
- **48 benchmarks** cobrindo todas as camadas da aplicação
- **Golden files** para comparação histórica
- **Detecção automática de regressões** com thresholds configuráveis
- **Métricas de percentil** (p50, p95, p99)
- **Integração CI/CD** com GitHub Actions

##  Comandos Disponíveis

### Executar Benchmarks

```bash
# Todos os benchmarks
make bench

# Benchmark específico
make bench-run name=BenchmarkDashboardRendering

# Com profiling (CPU + Memory)
make bench-profile name=BenchmarkFTS5Search

# Listar golden files disponíveis
make bench-list
```

### Comparação e Regressão

```bash
# Salvar resultados atuais como baseline
make bench-save

# Comparar com baseline (threshold padrão: 10%)
make bench-compare

# Comparar com threshold customizado
make bench-compare THRESHOLD=15

# Verificação rápida (apenas regressões)
make bench-check

# Limpar artefatos
make bench-clean
```

##  Estrutura de Golden Files

Os golden files são armazenados em `test/benchmarks/golden/` e contêm:

```json
{
  "version": "1.0",
  "timestamp": "2026-02-26T22:30:00Z",
  "go_version": "go1.24+",
  "cpu": "Intel(R) Core(TM) i7-10750H CPU @ 2.60GHz",
  "benchmarks": [
    {
      "name": "BenchmarkDashboardRendering",
      "iterations": 39552,
      "ns_per_op": 28576,
      "mem_allocs_per_op": 52,
      "mem_bytes_per_op": 5815,
      "p50_ns": 27500,
      "p95_ns": 32000,
      "p99_ns": 35000
    }
  ]
}
```

##  Categorias de Benchmarks

### 1. Middleware (2 benchmarks)
- `BenchmarkRequireAuthMiddleware` - Autenticação com sessão
- `BenchmarkSessionLookup` - Performance de sessão SCS

### 2. SSE Broker (3 benchmarks)
- `BenchmarkBroadcastScalability` - Escalabilidade (10, 100, 1000 clientes)
- `BenchmarkBroadcastToUser` - Broadcast direcionado
- `BenchmarkSSEClientRegistration` - Register/Unregister

### 3. Validator & Policies (2 benchmarks)
- `BenchmarkInputValidation` - Validação de structs
- `BenchmarkPostPolicyChecks` - Policy ABAC

### 4. Database Queries (4 benchmarks)
- `BenchmarkUserAuthentication` - GetUserByEmail/ByID
- `BenchmarkTenantIsolation` - Multi-tenant queries
- `BenchmarkPaginationLargeDataset` - Paginação (10K registros)
- `BenchmarkReadWriteContention` - Concorrência 90/10

### 5. Template Rendering (2 benchmarks)
- `BenchmarkComponentRendering` - Dashboard
- `BenchmarkTemplateWithLoops` - Listas (10, 50, 100 items)

### 6. Worker / Job Queue (7 benchmarks)
- `BenchmarkJobQueueOperations` - Create/Pick/Complete
- `BenchmarkConcurrentJobProcessing` - Workers concorrentes
- `BenchmarkJobIdempotency` - Idempotency keys
- `BenchmarkZombieJobRecovery` - Recuperação de jobs
- `BenchmarkJobPriorityScheduling` - FIFO scheduling

### 7. Comparativos (2 benchmarks)
- `BenchmarkBcryptCosts` - Hash costs 10, 12, 14
- `BenchmarkIdempotencyChecks` - Verificação de jobs

##  CI/CD Integration

O workflow de benchmarks no GitHub Actions:

1. **Executa benchmarks** em cada PR
2. **Compara com baseline** armazenada
3. **Comenta no PR** com relatório completo
4. **Falha se regressão > 10%**
5. **Atualiza golden file** no merge para main

### Exemplo de Saída no PR

```
##  Benchmark Results

================================================================================
                        BENCHMARK COMPARISON REPORT
================================================================================

Baseline:  2026-02-26T22:30:00Z
Current:   2026-02-27T01:39:15Z
Threshold: 10.0%

SUMMARY
--------------------------------------------------------------------------------
Total Benchmarks: 25
  Regressions:    0
  Improvements:   2
  Stable:         23

RESULT: PASSED - No regressions detected
================================================================================
```

##  Interpretando Resultados

### Métricas Chave

| Métrica | O que indica | Bom valor |
|---------|--------------|-----------|
| ns/op | Latência por operação | Menor é melhor |
| B/op | Alocação de memória | Menor é melhor |
| allocs/op | Número de alocações | Menor é melhor |
| p50 | Latência mediana | Representa caso típico |
| p95 | Latência 95% | Casos mais lentos |
| p99 | Latência 99% | Pior caso (exceto outliers) |

### Quando Investigar

- **Regressão > 10%**: Investigar causa raiz
- **p99 >> p50**: Possível contenção ou GC pressure
- **allocs/op alto**: Oportunidade de otimização
- **Memória crescendo**: Possível memory leak

##  Adicionando Novos Benchmarks

1. Crie função `BenchmarkXyz(b *testing.B)` em `performance_test.go`
2. Use `b.ReportAllocs()` para métricas de memória
3. Use `b.ResetTimer()` após setup
4. Adicione entrada correspondente no golden file
5. Execute `make bench-save` para atualizar baseline

### Exemplo

```go
func BenchmarkMyFeature(b *testing.B) {
    _, queries := setupSharedTestDB()
    
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Código a benchmark
        _, _ = queries.SomeOperation(ctx, params)
    }
}
```

##  Troubleshooting

### "No baseline found"
Execute `make bench-save` para criar baseline inicial.

### "Regressions detected"
1. Execute `make bench-compare` para ver detalhes
2. Identifique benchmarks com regressão
3. Investigue causa (código, dados, ambiente)
4. Se esperado, atualize baseline com `make bench-save`

### Benchmarks falhando localmente
1. Limpe artefatos: `make bench-clean`
2. Verifique espaço em disco
3. Feche outros programas pesados
4. Execute novamente

##  Referências

- [Go Testing Package](https://pkg.go.dev/testing)
- [Writing Go Benchmarks](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Benchmarking and Profiling Go Programs](https://blog.golang.org/profiling-go-programs)
