import json5
import json
import traceback
from typing import List, Optional, AsyncGenerator

from fastapi import FastAPI, Form, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel
from google.genai.types import Part

from core.agent import EGO
from core.prompts import *
from core.llm_backend import GeminiBackend
from core.tools import EgoSearch, EgoCalc, EgoWiki, EgoCode, AlterEgo

try:
    app = FastAPI()
    backend = GeminiBackend()
    tools = [ EgoSearch(backend=backend), AlterEgo(backend=backend), EgoCalc(), EgoWiki(), EgoCode() ]
    ego_instance = EGO(backend=backend, tools=tools)
    print("--- EGO Python API успешно инициализирован (SDK: google-genai) ---")
except Exception as e:
    print(f"!!! КРИТИЧЕСКАЯ ОШИБКА ИНИЦИАЛИЗАЦИИ: {e} !!!"); traceback.print_exc()

class EgoRequest(BaseModel):
    query: str
    mode: str
    chat_history: str = ""
    thoughts_history: str = ""
    custom_instructions: Optional[str] = None

class ToolExecutionRequest(BaseModel):
    query: str

def _get_thought_prompt(request_mode: str) -> str:
    return SEQUENTIAL_THINKING_PROMPT_RU_HEAVY if request_mode == "heavy" else SEQUENTIAL_THINKING_PROMPT_RU_FAST

def _get_synthesis_prompt(request_mode: str) -> str:
    return FINAL_SYNTHESIS_PROMPT_RU_HEAVY if request_mode == "heavy" else FINAL_SYNTHESIS_PROMPT_RU_FAST

@app.post("/generate_thought")
async def generate_thought(
    request_data: str = Form(...),
    files: List[UploadFile] = File(default=[])
):
    try:
        request = EgoRequest.parse_raw(request_data)
        
        prompt_parts_from_files = []
        if files:
            print(f"--- /generate_thought: Получено {len(files)} файлов через multipart/form-data. ---")
            for file in files:
                raw_bytes = await file.read()
                part = Part.from_bytes(data=raw_bytes, mime_type=file.content_type)
                prompt_parts_from_files.append(part)
        
        prompt_parts = [request.query] + prompt_parts_from_files
        
        sys_prompt_template = _get_thought_prompt(request.mode)
        sys_inst = sys_prompt_template.format(
            chat_history=request.chat_history, thoughts_history=request.thoughts_history, user_query=request.query)

        response_text, usage = await ego_instance.backend.generate(
            prompt_parts=prompt_parts, temp=0.7, sys_inst=sys_inst)
        
        json_str = ego_instance._extract_json_from_response(response_text)
        thought_json = json5.loads(json_str)

        return {"thought": thought_json, "usage": usage}
    except Exception as e:
        print(f"!!! Ошибка в /generate_thought: {e} !!!"); traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/execute_tool/{tool_name}")
async def execute_tool(tool_name: str, request: ToolExecutionRequest):
    try:
        tool = ego_instance.tools.get(tool_name)
        if not tool:
            raise HTTPException(status_code=404, detail=f"Инструмент '{tool_name}' не найден.")
        result = await tool.use(request.query)
        return {"result": str(result)}
    except Exception as e:
        print(f"!!! Ошибка в /execute_tool/{tool_name}: {e} !!!"); traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/synthesize_stream")
async def synthesize_stream(request: EgoRequest):
    async def event_generator() -> AsyncGenerator[str, None]:
        try:
            prompt_parts = [request.query] 
            synth_prompt_template = _get_synthesis_prompt(request.mode)
            sys_inst = synth_prompt_template.format(
                custom_instructions=request.custom_instructions or "",
                chat_history=request.chat_history,
                thoughts_history=request.thoughts_history,
                user_query=request.query,
            )
            async for text_chunk in ego_instance.backend.generate_stream(prompt_parts, temp=0.8, sys_inst=sys_inst):
                sse_event = {"type": "chunk", "data": {"text": text_chunk}}
                json_event = json.dumps(sse_event)
                yield f"data: {json_event}\n\n"
        except Exception as e:
            print(f"!!! Ошибка в генераторе synthesize_stream: {e} !!!"); traceback.print_exc()
            error_event = {"type": "error", "data": {"message": str(e)}}
            json_event = json.dumps(error_event)
            yield f"data: {json_event}\n\n"

    headers = {"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "Connection": "keep-alive", "X-Accel-Buffering": "no"}
    return StreamingResponse(event_generator(), headers=headers)