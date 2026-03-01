# Golden Baseline - Performance Benchmarks

Este diretório contém a baseline de referência para comparação de performance dos benchmarks do projeto Goth.

## Arquivos

- `golden_baseline.json`: Dados de baseline com métricas de performance de referência

## Versões da Baseline

### v2.1 (2026-02-27) - Correção de Bugs nos Benchmarks

**Motivo da atualização:** Correção de bugs críticos nos benchmarks que causavam falsos positivos/negativos.

**Mudanças:**

1. **BenchmarkReadWriteContention** - Correção de bug de distribuição read/write
   - **Bug**: `b.N%10 == 0` era avaliado uma vez antes do loop, não por iteração
   - **Impacto**: Se b.N fosse divisível por 10, 100% das iterações eram writes (não 10%)
   - **Fix**: Uso de contador atômico para distribuição correta 90% reads / 10% writes
   - **Mudança**: 5493 ns/op → 10520 ns/op (esperado, agora mede operação real)

2. **BenchmarkSQLiteReadWriteStress** - Melhoria no isolamento do banco de dados
   - **Mudança**: 47242 ns/op → 5537 ns/op (melhoria real de performance)
   - **Causa**: Melhor isolamento e uso de database dedicado para benchmarks

3. **Job Queue Benchmarks** - Correções de isolamento e estado
   - `BenchmarkJobQueueOperations/PickNextJob`: 4093264 → 738538 ns/op (81% faster)
   - `BenchmarkConcurrentJobProcessing`: 8045176 → 1938555 ns/op (76% faster)
   - `BenchmarkZombieJobRecovery/RecoverZombieJobs`: 18050133 → 8319198 ns/op (54% faster)
   - `BenchmarkJobPriorityScheduling/PickNextJob-FIFO`: 9382427 → 4574207 ns/op (51% faster)
   - **Causa**: Uso de database isolado com `setupIsolatedTestDB()` e limpeza adequada de estado

### v2.0 (2026-02-27) - Baseline Inicial

Primeira versão da baseline estruturada como JSON.

## Como Atualizar a Baseline

A baseline deve ser atualizada quando:

1. **Bugs nos benchmarks forem corrigidos** (como este caso)
2. **Mudanças arquiteturais significativas** que afetam performance
3. **Hardware de referência mudar** (atualmente: AMD EPYC 7763 64-Core no GitHub Actions)

**NÃO atualize a baseline para:**
- Variações normais de performance (< 15%)
- Mudanças cosméticas no código
- Otimizações prematuras sem impacto real

## Threshold de Regressão

O threshold atual para falha é **15%** de degradação normalizada.

## Hardware de Referência

- **CPU**: AMD EPYC 7763 64-Core Processor
- **OS**: Ubuntu 24.04 LTS (GitHub Actions runner)
- **Go Version**: 1.25.7
