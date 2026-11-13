# Diseño: bloques que se apilan solos y panel a todo el ancho

## Intención

El cliente quiere colocar los bloques con más libertad. Hoy, un bloque largo (Home Assistant) fija la altura de su «fila», y los cortos que tiene al lado (Portainer) dejan un hueco debajo hasta la fila siguiente. Quiere que dos bloques cortos puedan ir uno encima de otro al lado de uno largo. Además quiere ver más bloques uno al lado del otro y aprovechar todo el ancho del monitor: hoy el panel no pasa de 92rem y queda centrado.

**Causa actual.** El editor guarda para cada bloque `x`, `y`, `w` y `h` en una rejilla de 12 columnas, donde `h` son unidades de 28 px que nadie relaciona con el contenido: todos los bloques nacen con `h = 10`. El panel pinta un CSS grid con `grid-row: y / span h` y filas `auto`, así que el contenido real estira filas compartidas y empuja a los bloques de debajo aunque no estén en sus columnas.

## Objetivos

- En el panel, cada bloque queda justo debajo del último bloque de sus columnas, según la altura real de su contenido. No hay huecos verticales salvo el espacio entre bloques.
- El editor muestra lo mismo: alturas reales, y los bloques suben solos al soltar. Solo se elige la anchura, las columnas y el orden.
- El panel usa todo el ancho de la ventana, con su margen lateral, para que quepan tres, cuatro o más bloques por fila en un monitor ancho.
- Anchos útiles y fáciles de elegir: cuarto, tercio, mitad, dos tercios y entero.

## Fuera de alcance

- Elegir la altura de un bloque. Desaparece como decisión del usuario.
- Bloques que cambian de contenido según su tamaño (versiones compactas). Si un bloque estrecho no cabe, su contenido se reparte en más líneas, como hoy.
- Cambios en el esquema de la base de datos.

## Lo que lo haría fracasar

- Que el panel salte o parpadee cada pocos segundos al actualizarse los datos.
- Que el móvil empeore: debe seguir siendo una columna, en el orden del editor.
- Que un panel guardado antes se vea desordenado tras actualizar.

## Enfoques considerados

1. **Columnas que se apilan solas (elegido por el cliente).** Cada bloque conserva columnas, ancho y orden; en vertical se apila según la altura real. Libertad de colocación sin calcular alturas. Si un bloque crece, los de debajo bajan.
2. **Alturas fijas como en el editor.** Control total, pero con contenido recortado o con scroll si crece, y alturas que reajustar a mano.
3. **Columnas fijas (2 o 3 listas).** Lo más simple, pero sin anchos libres.

## Diseño

### Modelo

Sin cambios en la base de datos ni en la API: se siguen guardando `x`, `y`, `w` y `h` en 12 columnas.

- `x` y `w`: columnas del bloque, como hoy.
- `y`: solo da el **orden** (primero por `y`, después por `x`).
- `h`: la calcula el editor a partir de la altura real del contenido. El panel no la usa para colocar. Se sigue guardando porque la validación de solapes de la API y el editor la necesitan.

### Colocación en el panel («horizonte»)

Una función pura, `stack(widgets, alturas, gap)`, recorre los bloques en orden y, para cada uno, calcula su `top` como el máximo del fondo ya ocupado en sus columnas (`x … x+w-1`); después sube ese fondo a `top + altura + gap`. Devuelve `top` por bloque y la altura total.

- El contenedor del panel es `position: relative` con la altura total; cada bloque, `position: absolute` con `left` y `width` por columnas y el `top` calculado.
- Las alturas se miden con un `ResizeObserver` por bloque. Al cambiar una, se recalcula todo en el mismo fotograma. Sin animaciones de posición, para que las actualizaciones en vivo no se noten como movimiento.
- Antes de la primera medida, los bloques se muestran en flujo normal (una columna) para no dejar la página vacía.
- **Anchos según la pantalla:**
  - Menos de 900 px: una sola columna, en orden, como hoy.
  - De 900 a 1400 px: los bloques de menos de media fila pasan a media fila y, como así ya no caben en sus columnas, se recolocan por orden de izquierda a derecha en dos mitades (columnas 0 y 6), como hace hoy `reflow` en el editor, antes de apilar. Los de media fila o más conservan su ancho y se colocan igual, por orden.
  - Más de 1400 px: el ancho y las columnas elegidos.

  El editor siempre trabaja con la disposición de escritorio (12 columnas, sin estas reglas). Los umbrales son una decisión del constructor si al probar no encajan, y la regla se documenta.

### Ancho del panel

`.lookout-main` deja de tener `max-width: 92rem`: ocupa toda la ventana con el margen lateral actual. El editor (`.panel-wide`) también, para que las proporciones coincidan.

### Editor

- `react-grid-layout` sigue con su compactación vertical por defecto, que es el mismo apilado que el panel.
- La altura `h` de cada bloque se calcula a partir de la vista previa: `ceil((alto_px + margen) / (rowHeight + margen))`, medido con `ResizeObserver`. No se puede redimensionar en vertical: el tirador de la esquina pasa a ser lateral (`e`) y solo cambia el ancho.
- **Las alturas medidas no cuentan como cambios.** El estado «sin guardar» compara las páginas sin `h`, así que abrir el editor o que lleguen datos nuevos no lo activa. La `h` medida se envía cuando se guarda por otros motivos.
- Los botones «Más estrecho» y «Más ancho» recorren los anchos 3, 4, 6, 8 y 12, saltando al siguiente de la lista desde cualquier ancho. El tirador lateral permite cualquier ancho de 3 a 12: la lista es una comodidad de los botones, no una regla, y la API no la valida.
- El ancho mínimo del editor pasa de 2 a 3 columnas (`minW`). La API mantiene el mínimo de 2 para no rechazar paneles guardados antes.
- Los bloques nuevos nacen con ancho 4 (un tercio), en el editor y en `placeNewConnection`.

La vista previa del editor mide el bloque con el ancho del editor, que puede no coincidir con el del panel. Por eso la altura del editor es aproximada. El panel siempre mide la suya.

### Paneles ya guardados

No necesitan migración. Su orden por `(y, x)` y sus columnas se respetan, y la altura guardada se ignora en el panel. Un bloque antiguo de 2 columnas se pinta con 2 hasta que se cambie su ancho en el editor; la API lo sigue aceptando al guardar.

## Pruebas y documentación

- Pruebas de Go: ancho por defecto (4) de una conexión nueva y su colocación.
- Prueba unitaria, o en E2E si no hay pruebas de frontend, del apilado: dos bloques cortos en las columnas 1-6 y uno largo en 7-12 dan dos tops distintos a la izquierda y ninguno a la derecha.
- E2E: en el panel real con simuladores, un bloque corto colocado bajo otro corto, al lado de uno largo, empieza donde acaba el de arriba y no donde acaba el largo. El panel ocupa más de 92rem en una ventana ancha.
- Capturas regeneradas en los dos idiomas.
- Documentación: 01 (requisito de colocación), 03 (reglas de altura, ancho y anchos disponibles), 05 (DEC nueva, que sustituye en parte a DEC-38), 06 (hito) y los dos manuales («Organizar el panel»).

## Decisiones del constructor

Las marcadas como tales arriba: los umbrales de pantalla, el ancho por defecto (4) y la secuencia de anchos (3, 4, 6, 8 y 12). Si al probar con el panel real alguna no funciona, se ajusta y se documenta. Tras la revisión del diseño se decidieron también: la recolocación en dos mitades en pantallas medianas, que las alturas medidas no cuenten como cambios, que la API mantenga el mínimo de 2 y que la lista de anchos sea solo de los botones.
