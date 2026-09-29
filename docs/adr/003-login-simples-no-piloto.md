# 003 - Login com e-mail e senha no piloto local

**Status:** Accepted

## Context
O Hub precisa saber quem é cada pessoa (responsável, reserva, permissões). O piloto roda só no computador do Henry.

## Decision
Login com e-mail e senha (bcrypt), sessão em cookie `HttpOnly` guardada no Postgres. O primeiro acesso cria o administrador. Depois, só administradores cadastram pessoas, com três perfis: administrador, criador (registra apps) e usuário (usa apps). Em desenvolvimento existe `POST /api/dev/login`, registrado apenas com `DEV_MODE`.

## Rationale
É o mais simples para um piloto sem internet e sem configuração externa.

## Trade-offs
**Pros:**
- Nenhum serviço externo.

**Cons:**
- Senha própria não é adequada para produção. Antes do Módulo 2, trocar por Google/Microsoft ou link mágico por e-mail.

## Alternatives Considered
- Login Google/Microsoft: fora do escopo do piloto (PRD).
- Sem login: descartado, porque responsável, reserva e permissões precisam de identidade.
