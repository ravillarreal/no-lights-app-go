@AGENTS.md

## Claude Code — Workflow

### Modo de trabajo
- Usa **plan mode** (`Shift+Tab`) para cambios que tocan varios archivos o cuyo enfoque no está claro. Para cambios de una línea o de alcance obvio, hazlo directo.
- Antes de implementar features grandes, **entrevístame** usando la herramienta de preguntas: indaga sobre implementación, edge cases, tradeoffs y cosas que no haya considerado. Luego escribe un `SPEC.md` autocontenido y ejecútalo en una sesión limpia.
- **Verifica siempre tu trabajo**: corre los tests, el build o una captura, y muéstrame la evidencia (salida del comando), no solo "hecho".

### Especificidad de los requerimientos
- **Sé más específico** cuando el alcance es claro, el cambio es pequeño, o quiero un formato exacto.
- **Sé menos específico** cuando estoy explorando, no conozco la solución, o prefiero que propongas opciones que yo no imaginé. En ese caso, plantea alternativas antes de implementar.

### Convenciones del proyecto
- Los tests corren con `go test ./...`; el build con `go build ./...`; `gofmt` y `go vet` se exigen en CI.
- Los endpoints nuevos deben sumar su aserción de `X-Query-Count` en `internal/api/server_test.go` y documentar su query count esperado.
- No quites ni modifiques tests para "arreglar" un fallo — el fallo es la señal.
