# Voice Assistant Demo

Un asistente de voz local que utiliza **Whisper** (OpenAI) para reconocimiento de voz, un clasificador de intenciones y **Gemini** como fallback, con una interfaz gráfica en tkinter.

## Características

- 🎤 Reconocimiento de voz con Whisper (modelo local)
- 🏠 Clasificación de intenciones para controlar dispositivos del hogar
- 🌐 Fallback a Google Generative AI (Gemini) para consultas más complejas
- 🖥️ Interfaz gráfica intuitiva con tkinter
- 🔊 Captura de audio en tiempo real

## Requisitos Previos

- **Python 3.8+**
- **pip** (gestor de paquetes de Python)
- Conexión a internet (para descargar modelos y acceder a Gemini API)

## Instalación de Dependencias

### Opción 1: Instalación rápida (recomendado)

Ejecuta el siguiente comando en la terminal desde el directorio del proyecto:

```bash
pip install openai-whisper google-generativeai sounddevice soundfile numpy torch
```

### Opción 2: Instalación paso a paso

Si prefieres instalar las dependencias una por una:

```bash
# Reconocimiento de voz
pip install openai-whisper

# API de Google Generative AI (Gemini)
pip install google-generativeai

# Captura de audio
pip install sounddevice soundfile

# Computación numérica
pip install numpy

# Deep Learning (requerido por Whisper)
pip install torch
```

## Descripción de Dependencias

| Paquete | Versión | Propósito |
|---------|---------|----------|
| `openai-whisper` | Latest | Modelo de reconocimiento de voz de OpenAI |
| `google-generativeai` | Latest | API de Google Generative AI (Gemini) |
| `sounddevice` | Latest | Captura de audio desde dispositivos |
| `soundfile` | Latest | Lectura/escritura de archivos de audio |
| `numpy` | Latest | Operaciones numéricas y manejo de arrays |
| `torch` | Latest | Framework de Deep Learning (PyTorch) |

## Uso

Ejecuta el script de prueba:

```bash
python test.py
```

Esto abrirá una interfaz gráfica donde podrás:
1. Grabar comandos de voz
2. Procesar la entrada a través de Whisper
3. Clasificar la intención del usuario
4. Ejecutar acciones del hogar inteligente

## Acciones Soportadas

- **light_on**: Encender la luz
- **light_off**: Apagar la luz
- **light_color**: Cambiar el color de la luz
- **check_temperature_humidity**: Comprobar temperatura y humedad
- **set_temperature**: Modificar temperatura

## Habitaciones Soportadas

- Salón
- Cocina
- Baño
- Habitación / Dormitorio
- Oficina
- Comedor
- Pasillo

## Configuración

Antes de ejecutar el script, verifica los siguientes parámetros en `test.py`:

- `GEMINI_API_KEY`: Tu clave de API de Google Generative AI
- `GEMINI_MODEL`: Modelo a utilizar (por defecto: "gemini-2.5-flash")
- `WHISPER_MODEL`: Modelo de Whisper (por defecto: "base")
- `SAMPLE_RATE`: Frecuencia de muestreo en Hz (por defecto: 16000)
- `MAX_SECONDS`: Duración máxima de grabación (por defecto: 30 segundos)

## Solución de Problemas

### Error: "ModuleNotFoundError: No module named 'whisper'"

Instala las dependencias ejecutando:
```bash
pip install -r requirements.txt
```

O instala manualmente:
```bash
pip install openai-whisper google-generativeai sounddevice soundfile numpy torch
```

### Error: "No audio device found"

Asegúrate de que tu micrófono está conectado y reconocido por el sistema.

### Error: "GEMINI_API_KEY not valid"

Verifica que tu clave de API de Google Generative AI sea correcta y esté activa.

## Notas Técnicas

- El modelo Whisper se descargará automáticamente la primera vez que se ejecute
- El tamaño del modelo base es aproximadamente 140 MB
- Se requiere una GPU para mejor rendimiento (opcional, funcionará con CPU)

## Licencia

Proyecto educativo para la UPF - Xarxes de sensors sense fils

---

**Autor**: Equipo de Proyecto Final  
**Institución**: Universitat Pompeu Fabra (UPF)  
**Fecha**: 2026
