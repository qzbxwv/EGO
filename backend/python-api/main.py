import json5
import json
import traceback
from typing import List, Optional, AsyncGenerator

from fastapi import FastAPI, Form, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel
from google.genai.types import Part

from core.agent import EGO
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

@app.post("/generate_thought")
async def generate_thought(
    request_data: str = Form(...),
    files: List[UploadFile] = File(default=[])
):
    try:
        request = EgoRequest.parse_raw(request_data)
        
        prompt_parts_from_files = []
        if files:
            print(f"--- /generate_thought: Получено {len(files)} файлов. ---")
            for file in files:
                raw_bytes = await file.read()
                part = Part.from_bytes(data=raw_bytes, mime_type=file.content_type)
                prompt_parts_from_files.append(part)
        
        thought_json, usage = await ego_instance.generate_thought(
            query=request.query,
            mode=request.mode,
            chat_history=request.chat_history,
            thoughts_history=request.thoughts_history,
            prompt_parts_from_files=prompt_parts_from_files
        )

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
async def synthesize_stream(
    request_data: str = Form(...),
    files: List[UploadFile] = File(default=[])
):
    request = EgoRequest.parse_raw(request_data)
    
    prompt_parts_from_files = []
    if files:
        print(f"--- /synthesize_stream: Чтение {len(files)} файлов в память... ---")
        for file in files:
            raw_bytes = await file.read()
            part = Part.from_bytes(data=raw_bytes, mime_type=file.content_type)
            prompt_parts_from_files.append(part)
            await file.close() 
    
    async def event_generator() -> AsyncGenerator[str, None]:
        try:
            async for text_chunk in ego_instance.synthesize_stream(
                query=request.query,
                mode=request.mode,
                chat_history=request.chat_history,
                thoughts_history=request.thoughts_history,
                custom_instructions=request.custom_instructions,
                prompt_parts_from_files=prompt_parts_from_files
            ):
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