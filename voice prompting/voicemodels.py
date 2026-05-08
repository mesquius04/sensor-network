"""
Voice Assistant Demo — Local Whisper + Intent Classifier + Gemini fallback
=========================================================================
pip install openai-whisper google-generativeai sounddevice soundfile numpy torch
"""

import tkinter as tk
from tkinter import ttk, scrolledtext
import threading
import json
import tempfile
import os
import re
import numpy as np
import sounddevice as sd
import soundfile as sf
import whisper

# =========================
# CONFIG
# =========================

GEMINI_API_KEY = "AIzaSyCYTKTAs2XeGkHTcXqm0jWvUsjmXDiCuOc"
GEMINI_MODEL = "gemini-2.5-flash"

WHISPER_MODEL = "base"

SAMPLE_RATE = 16000
MAX_SECONDS = 30

LOCAL_CONFIDENCE_THRESHOLD = 0.55

# =========================
# INTENTS HARDCODEADOS
# =========================

ROOMS = {

    "salón": "living_room",
    "salon": "living_room",

    "cocina": "kitchen",

    "baño": "bathroom",
    "bano": "bathroom",

    "habitación": "bedroom",
    "habitacion": "bedroom",
    "cuarto": "bedroom",
    "dormitorio": "bedroom",

    "oficina": "office",

    "comedor": "dining_room",

    "pasillo": "hallway"
}

INTENTS = [
    {
        "action": "light_on",
        "description": "Encender la luz"
    },
    {
        "action": "light_off",
        "description": "Apagar la luz"
    },
    {
        "action": "light_color",
        "description": "Cambiar el color de la luz"
    },
    {
        "action": "check_temperature_humidity",
        "description": "Comprobar temperatura y humedad"
    },
    {
        "action": "set_temperature",
        "description": "Modificar temperatura"
    }
]

VALID_ACTIONS = {intent["action"] for intent in INTENTS}

KEYWORDS = {
    "light_on": [
        "enciende", "encender", "prende", "prender", "activa", "activar",
        "pon la luz", "enciende la luz", "luces encendidas"
    ],

    "light_off": [
        "apaga", "apagar", "desactiva", "desactivar",
        "quita la luz", "apaga la luz", "luces apagadas"
    ],

    "light_color": [
        "color", "rojo", "roja", "azul", "verde", "amarillo", "amarilla",
        "blanco", "blanca", "morado", "morada", "violeta", "rosa",
        "naranja", "cian", "lila"
    ],

    "check_temperature_humidity": [
        "temperatura", "humedad", "qué temperatura", "que temperatura",
        "cuánta humedad", "cuanta humedad", "cuántos grados", "cuantos grados",
        "hace calor", "hace frío", "hace frio", "ambiente"
    ],

    "set_temperature": [
        "pon", "poner", "ajusta", "ajustar", "sube", "subir",
        "baja", "bajar", "grados", "grado", "calefacción",
        "calefaccion", "aire acondicionado", "termostato"
    ]
}

COLORS = {
    "rojo": "red",
    "roja": "red",
    "azul": "blue",
    "verde": "green",
    "amarillo": "yellow",
    "amarilla": "yellow",
    "blanco": "white",
    "blanca": "white",
    "morado": "purple",
    "morada": "purple",
    "violeta": "violet",
    "rosa": "pink",
    "naranja": "orange",
    "cian": "cyan",
    "lila": "purple"
}

# =========================
# LOAD WHISPER
# =========================

print("Loading Whisper model...")
whisper_model = whisper.load_model(WHISPER_MODEL)
print("Whisper loaded.")

# =========================
# SYSTEM PROMPT
# =========================
SYSTEM_PROMPT = f"""
Eres un clasificador de intents para un sistema domotico.

Debes seleccionar UNICAMENTE la accion mas probable.

ACCIONES DISPONIBLES:
{json.dumps(INTENTS, ensure_ascii=False, indent=2)}

HABITACIONES DISPONIBLES:
{json.dumps(list(set(ROOMS.values())), ensure_ascii=False, indent=2)}

Devuelve SOLO JSON valido.

Formato:

{{
  "action": "light_on",
  "room": "living_room",
  "confidence": 0.97,
  "parameters": {{
      "color": null,
      "temperature": null
  }},
  "source": "gemini"
}}

Reglas:
- SOLO puedes usar acciones válidas.
- SOLO puedes usar habitaciones válidas.
- NO inventes habitaciones nuevas.
- Si no se menciona habitación usa null.
- confidence entre 0 y 1.
"""

# =========================
# WHISPER LOCAL
# =========================

def transcribe_audio(filepath: str) -> str:
    result = whisper_model.transcribe(
        filepath,
        language="es"
    )

    return result["text"].strip()

# =========================
# LOCAL INTENT CLASSIFIER
# =========================

def extract_color(text: str):
    text = text.lower()

    for spanish_color, normalized_color in COLORS.items():
        if spanish_color in text:
            return normalized_color

    return None

def extract_room(text: str):

    text = text.lower()

    for spanish_room, normalized_room in ROOMS.items():

        if spanish_room in text:
            return normalized_room

    return None

def extract_temperature(text: str):
    text = text.lower()

    patterns = [
        r"(\d+(?:[.,]\d+)?)\s*grados",
        r"(\d+(?:[.,]\d+)?)\s*º",
        r"(\d+(?:[.,]\d+)?)\s*c",
        r"a\s*(\d+(?:[.,]\d+)?)"
    ]

    for pattern in patterns:
        match = re.search(pattern, text)

        if match:
            value = match.group(1).replace(",", ".")

            try:
                return float(value)
            except ValueError:
                return None

    return None

def local_intent_classification(text: str):

    text = text.lower().strip()

    scores = {}

    for action, keywords in KEYWORDS.items():

        score = 0

        for keyword in keywords:

            if keyword in text:
                score += 1

        scores[action] = score

    color = extract_color(text)

    temperature = extract_temperature(text)

    room = extract_room(text)

    # Heurísticas extra
    if color is not None:
        scores["light_color"] += 2

    if temperature is not None:
        scores["set_temperature"] += 2

    question_words = [
        "qué", "que",
        "cuál", "cual",
        "cuánto", "cuanto",
        "dime"
    ]

    if any(word in text for word in question_words):

        if (
            "temperatura" in text
            or "humedad" in text
            or "grados" in text
        ):
            scores["check_temperature_humidity"] += 3

    best_action = max(scores, key=scores.get)

    best_score = scores[best_action]

    total_score = sum(scores.values())

    if best_score == 0 or total_score == 0:
        return None

    confidence = best_score / total_score

    result = {
        "action": best_action,

        "room": room,

        "confidence": round(confidence, 2),

        "parameters": {
            "color": color if best_action == "light_color" else None,

            "temperature": (
                temperature
                if best_action == "set_temperature"
                else None
            )
        },

        "source": "local",

        "scores": scores
    }

    return result
def normalize_result(result: dict) -> dict:

    action = result.get("action")

    room = result.get("room")

    valid_rooms = set(ROOMS.values())

    if action not in VALID_ACTIONS:

        return {
            "action": "unknown",
            "room": None,
            "confidence": 0.0,
            "parameters": {
                "color": None,
                "temperature": None
            },
            "source": result.get("source", "unknown"),
            "error": "Invalid action returned"
        }

    if room not in valid_rooms:
        room = None

    parameters = result.get("parameters") or {}

    return {

        "action": action,

        "room": room,

        "confidence": float(result.get("confidence", 0.0)),

        "parameters": {

            "color": parameters.get("color"),

            "temperature": parameters.get("temperature")
        },

        "source": result.get("source", "unknown"),

        "scores": result.get("scores")
    }

# =========================
# GEMINI FALLBACK
# =========================

def call_gemini(transcription: str) -> dict:
    import google.generativeai as genai

    genai.configure(api_key=GEMINI_API_KEY)

    model = genai.GenerativeModel(
        model_name=GEMINI_MODEL,
        system_instruction=SYSTEM_PROMPT,
    )

    response = model.generate_content(transcription)

    raw = response.text.strip()
    raw = raw.replace("```json", "")
    raw = raw.replace("```", "")
    raw = raw.strip()

    result = json.loads(raw)
    result["source"] = "gemini"

    return normalize_result(result)


def classify_command(transcription: str) -> dict:
    local_result = local_intent_classification(transcription)

    if local_result is not None:
        local_result = normalize_result(local_result)

        if local_result["confidence"] >= LOCAL_CONFIDENCE_THRESHOLD:
            return local_result

    gemini_result = call_gemini(transcription)

    if local_result is not None:
        gemini_result["local_candidate"] = local_result

    return gemini_result

# =========================
# DISPATCHER SIMULADO
# =========================
def dispatch_command(command: dict):

    action = command["action"]

    room = command.get("room")

    params = command.get("parameters", {})

    room_text = room if room else "default_room"

    if action == "light_on":

        message = f"Encendiendo luz en {room_text}"

    elif action == "light_off":

        message = f"Apagando luz en {room_text}"

    elif action == "light_color":

        color = params.get("color")

        message = (
            f"Cambiando luz de {room_text} "
            f"a color {color}"
        )

    elif action == "check_temperature_humidity":

        message = (
            f"Consultando temperatura y humedad "
            f"en {room_text}"
        )

    elif action == "set_temperature":

        temperature = params.get("temperature")

        message = (
            f"Configurando temperatura de "
            f"{room_text} a {temperature} grados"
        )

    else:

        message = "Acción desconocida"

    return {
        "executed": True,
        "message": message
    }

# =========================
# UI
# =========================

class VoiceAssistantApp(tk.Tk):

    def __init__(self):
        super().__init__()

        self.title("Voice Assistant")
        self.geometry("700x720")
        self.configure(bg="#0f0f0f")

        self._recording = False
        self._stop_event = threading.Event()
        self._frames = []

        self._build_ui()

    def _build_ui(self):

        tk.Label(
            self,
            text="Whisper + Intent Classifier",
            font=("Arial", 22, "bold"),
            bg="#0f0f0f",
            fg="white"
        ).pack(pady=20)

        btn_frame = tk.Frame(self, bg="#0f0f0f")
        btn_frame.pack(fill="x", padx=30)

        self.record_btn = tk.Button(
            btn_frame,
            text="Start Recording",
            font=("Arial", 14, "bold"),
            bg="#2563eb",
            fg="white",
            relief="flat",
            height=2,
            cursor="hand2",
            command=self._start_recording,
        )

        self.record_btn.pack(
            side="left",
            fill="x",
            expand=True,
            padx=(0, 6)
        )

        self.stop_btn = tk.Button(
            btn_frame,
            text="Stop",
            font=("Arial", 14, "bold"),
            bg="#374151",
            fg="#6b7280",
            relief="flat",
            height=2,
            cursor="hand2",
            state="disabled",
            command=self._stop_recording,
        )

        self.stop_btn.pack(
            side="left",
            fill="x",
            expand=True,
            padx=(6, 0)
        )

        self.status_label = tk.Label(
            self,
            text="Listo",
            font=("Courier New", 10),
            bg="#0f0f0f",
            fg="#6b7280"
        )

        self.status_label.pack(pady=(10, 0))

        self.progress = ttk.Progressbar(
            self,
            mode="indeterminate",
            length=600
        )

        tk.Label(
            self,
            text="Transcription",
            bg="#0f0f0f",
            fg="#9ca3af"
        ).pack(anchor="w", padx=30, pady=(16, 2))

        self.transcription_box = scrolledtext.ScrolledText(
            self,
            height=5,
            bg="#111827",
            fg="white",
            font=("Courier New", 11),
            relief="flat"
        )

        self.transcription_box.pack(fill="x", padx=30)

        tk.Label(
            self,
            text="JSON Response",
            bg="#0f0f0f",
            fg="#9ca3af"
        ).pack(anchor="w", padx=30, pady=(16, 2))

        self.json_box = scrolledtext.ScrolledText(
            self,
            height=12,
            bg="#0d1117",
            fg="#4ade80",
            font=("Courier New", 11),
            relief="flat"
        )

        self.json_box.pack(
            fill="both",
            expand=True,
            padx=30,
            pady=(0, 20)
        )

    # =========================
    # RECORDING
    # =========================

    def _start_recording(self):

        if self._recording:
            return

        self._recording = True
        self._frames = []
        self._stop_event = threading.Event()

        self.record_btn.configure(
            state="disabled",
            bg="#991b1b",
            text="Recording..."
        )

        self.stop_btn.configure(
            state="normal",
            bg="#dc2626",
            fg="white",
            text="Stop"
        )

        self._set_status(
            "Grabando... pulsa Stop cuando termines",
            "#f87171"
        )

        self.progress.pack(pady=(0, 6))
        self.progress.start(12)

        threading.Thread(
            target=self._record_stream,
            daemon=True
        ).start()

    def _stop_recording(self):
        self._stop_event.set()

    def _record_stream(self):

        def callback(indata, frames, time_info, status):
            self._frames.append(indata.copy())

            total = sum(len(f) for f in self._frames)

            if total >= MAX_SECONDS * SAMPLE_RATE:
                self._stop_event.set()

        with sd.InputStream(
            samplerate=SAMPLE_RATE,
            channels=1,
            dtype="float32",
            callback=callback,
        ):
            self._stop_event.wait()

        self.after(0, self._on_recording_done)

    # =========================
    # PIPELINE
    # =========================

    def _on_recording_done(self):

        self._set_status(
            "Transcribiendo con Whisper...",
            "#a855f7"
        )

        self.stop_btn.configure(
            state="disabled",
            bg="#374151",
            fg="#6b7280"
        )

        threading.Thread(
            target=self._run_pipeline,
            daemon=True
        ).start()

    def _run_pipeline(self):

        try:
            audio = np.concatenate(self._frames, axis=0)

            with tempfile.NamedTemporaryFile(
                suffix=".wav",
                delete=False
            ) as f:
                tmp_path = f.name

            sf.write(tmp_path, audio, SAMPLE_RATE)

            # =========================
            # WHISPER
            # =========================

            text = transcribe_audio(tmp_path)

            os.unlink(tmp_path)

            self._update_box(
                self.transcription_box,
                text
            )

            # =========================
            # LOCAL CLASSIFIER + GEMINI FALLBACK
            # =========================

            self.after(
                0,
                lambda: self._set_status(
                    "Clasificando intención...",
                    "#38bdf8"
                )
            )

            result = classify_command(text)

            execution_result = dispatch_command(result)

            final_output = {
                "transcription": text,
                "intent": result,
                "execution": execution_result
            }

            pretty = json.dumps(
                final_output,
                ensure_ascii=False,
                indent=2
            )

            self._update_box(
                self.json_box,
                pretty
            )

            self.after(
                0,
                lambda: self._set_status(
                    "Listo",
                    "#22c55e"
                )
            )

        except Exception as e:

            self._update_box(
                self.json_box,
                f"// Error:\n// {e}"
            )

            self.after(
                0,
                lambda: self._set_status(
                    f"Error: {e}",
                    "#ef4444"
                )
            )

        finally:

            self._recording = False

            self.after(
                0,
                self._reset_buttons
            )

    # =========================
    # HELPERS
    # =========================

    def _reset_buttons(self):

        self.record_btn.configure(
            state="normal",
            bg="#2563eb",
            text="Start Recording"
        )

        self.progress.stop()
        self.progress.pack_forget()

    def _set_status(self, msg: str, color: str = "#9ca3af"):

        self.status_label.configure(
            text=msg,
            fg=color
        )

    def _update_box(self, box, text: str):

        def _do():

            box.configure(state="normal")
            box.delete("1.0", "end")
            box.insert("1.0", text)
            box.configure(state="disabled")

        self.after(0, _do)

# =========================
# MAIN
# =========================

if __name__ == "__main__":

    app = VoiceAssistantApp()
    app.mainloop()