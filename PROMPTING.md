# Cómo hacer los mejores prompts a Claude

Guía práctica basada en la documentación oficial de Anthropic (prompt engineering + Claude Code best practices). El objetivo: un vocabulario y un flujo que puedas repetir en cada tarea.

---

## 1. La anatomía de un buen prompt

Cada tarea debería tocar estos bloques. No todos son obligatorios siempre, pero si falta uno y el resultado sale mal, suele ser el culpable.

```
1. OBJETIVO   — el resultado, no el método
2. CONTEXTO   — por qué y bajo qué restricciones
3. EJEMPLOS   — 3-5 muestras de lo que quieres (o referencias a patrones existentes)
4. FORMATO    — cómo quieres la salida (estructura, tags, JSON)
5. VERIFICACIÓN — cómo saber que está bien (tests, build, criterios)
```

**Regla de oro de Anthropic**: muéstrale tu prompt a un colega sin contexto de la tarea. Si se confunde, Claude también.

### Antes vs después

| Débil | Fuerte |
|-------|--------|
| "agrega tests para foo" | "escribe un test de foo cubriendo el caso donde el usuario está deslogueado, sin mocks" |
| "arregla el bug de login" | "login falla tras timeout de sesión. Mira el auth flow en src/auth/, especialmente el refresh de token. Escribe un test que reproduzca el fallo y luego arréglalo" |
| "haz un dashboard" | "mira cómo están hechos los widgets existentes en la home y sigue ese patrón. Sin librerías nuevas" |

---

## 2. ¿Más específico o menos específico?

Esta es la decisión que más cambia el resultado. La regla que sigue:

### Sé MÁS específico cuando…

- El alcance es claro y el cambio es pequeño ("si puedes describir el diff en una frase, no hagas plan")
- Necesitas un formato exacto (JSON, tags, estructura de respuesta)
- Quieres que siga un patrón existente en el código
- Estás arreglando un bug con síntoma conocido

### Sé MENOS específico cuando…

- Estás explorando y no sabes qué es posible
- **No conoces la solución** — y sospechas que Claude (con millones de ejemplos en su memoria) puede ver un camino mejor que el que tienes en tu cabeza
- Quieres que te muestre opciones que no imaginaste

> **Tu intuición es correcta y está documentada**: Anthropic dice que prompts vagos son útiles cuando exploras — *"what would you improve in this file?"* puede revelar cosas que no habrías pensado preguntar.

### La frase mágica para el caso "menos específico"

Cuando no sabes la solución y prefieres que Claude proponga:

```
"Aquí está lo que quiero lograr: [resultado]. Antes de implementar,
proponme 2-3 enfoques distintos con sus tradeoffs, y recomiéndame uno."
```

Esto te da el beneficio de su conocimiento sin comprometerte con una implementación equivocada.

---

## 3. El flujo de trabajo ideal

Para features grandes, el flujo que recomienda Anthropic (y que encaja con tu preferencia por las preguntas):

```
1. EXPLORAR   → plan mode. Lee el código, haz preguntas, no edites nada.
2. ENTREVISTA → Claude te hace preguntas (edge cases, tradeoffs, cosas que no pensaste).
3. SPEC       → se escribe SPEC.md autocontenido.
4. IMPLEMENTAR → sesión limpia, ejecuta el spec.
5. VERIFICAR  → tests/build/evidencia (no solo "hecho").
6. COMMIT     → mensaje descriptivo, PR.
```

### El prompt de entrevista (lo que te gusta)

```
Quiero construir [descripción breve]. Entrevístame en detalle usando tu
herramienta de preguntas. Indaga sobre implementación técnica, UI/UX,
edge cases, tradeoffs. No preguntes lo obvio; cava en las partes difíciles
que quizá no haya considerado. Sigue hasta cubrir todo, luego escribe un
SPEC.md completo.
```

**Por qué funciona**: un spec preciso paga más que mirar la implementación. El tiempo en hacerlo preciso ahorra correcciones después.

---

## 4. El lenguaje de las instrucciones

Vocabulario que cambia el comportamiento de Claude de forma predecible:

| Instrucción | Efecto |
|-------------|--------|
| "Cambia X para Y" (no "¿puedes sugerir…?") | Claude **actúa** en vez de sugerir |
| "Responde directo, sin preámbulo" | Elimina el "Aquí está… / Basado en…" |
| "Antes de terminar, verifica contra [criterios]" | Activa auto-chequeo |
| "Implementa los cambios, no solo los sugieras" | Por defecto pasa a acción |
| "Cita primero las partes relevantes del doc, luego responde" | Mejora respuestas sobre documentos largos |
| "Piensa en voz alta los pasos" | Activa razonamiento paso a paso |

### Estructura con XML

Para prompts que mezclan instrucciones + contexto + ejemplos, usa tags:

```xml
<instructions>…</instructions>
<context>…</context>
<example>…</example>
```

---

## 5. Anti-patrones (lo que falla)

1. **Sesión "cajón de sastre"**: mezclar tareas no relacionadas llena el contexto. → `/clear` entre tareas.
2. **Corregir en loop**: dos correcciones fallidas seguidas → contexto contaminado. → `/clear` y reescribe el prompt con lo aprendido.
3. **CLAUDE.md inflado**: demasiadas reglas → Claude ignora las importantes. → poda sin piedad.
4. **Confiar sin verificar**: Claude produce algo plausible con bugs en edge cases. → siempre pide verificación (tests, build, captura).
5. **Exploración infinita**: "investiga X" sin acotar → lee cientos de archivos. → acota o usa subagentes.

---

## 6. Resumen en una línea

> **Sé específico sobre el *qué* (resultado, formato, verificación); deja el *cómo* abierto cuando no conozcas la mejor solución.** Y para features grandes, deja que Claude te entreviste antes de escribir código.
