import json5
from typing import List, Dict, Any, Union, AsyncGenerator
from PIL import Image

from .prompts import (
    SEQUENTIAL_THINKING_PROMPT_EN_DEFAULT,
    SEQUENTIAL_THINKING_PROMPT_EN_DEEPER,
    SEQUENTIAL_THINKING_PROMPT_EN_RESEARCH,
    FINAL_SYNTHESIS_PROMPT_EN_DEFAULT,
    FINAL_SYNTHESIS_PROMPT_EN_DEEPER,
    FINAL_SYNTHESIS_PROMPT_EN_RESEARCH,
)
from .tools import Tool
from .llm_backend import LLMBackend


class EGO:
    def __init__(self, backend: LLMBackend, tools: List[Tool], max_thoughts: int = 100, max_retries: int = 3):
        self.backend = backend
        self.tools: Dict[str, Tool] = {tool.name: tool for tool in tools}
        self.max_thoughts = max_thoughts
        self.max_retries = max_retries
        
        self.THINKING_PROMPTS = {
            "default": SEQUENTIAL_THINKING_PROMPT_EN_DEFAULT,
            "deeper": SEQUENTIAL_THINKING_PROMPT_EN_DEEPER,
            "research": SEQUENTIAL_THINKING_PROMPT_EN_RESEARCH,
        }
        self.SYNTHESIS_PROMPTS = {
            "default": FINAL_SYNTHESIS_PROMPT_EN_DEFAULT,
            "deeper": FINAL_SYNTHESIS_PROMPT_EN_DEEPER,
            "research": FINAL_SYNTHESIS_PROMPT_EN_RESEARCH,
        }

    def _extract_json_from_response(self, text: str) -> str:
        try:
            first_brace = text.find('{')
            if first_brace == -1: return ""
            last_brace = text.rfind('}')
            if last_brace == -1 or last_brace < first_brace: return ""
            json_str = text[first_brace : last_brace + 1]
            return json_str.strip()
        except Exception:
            return ""

    async def generate_thought(self, query: str, mode: str, chat_history: str, thoughts_history: str, prompt_parts_from_files: List[Any]):
        prompt_template = self.THINKING_PROMPTS.get(mode, self.THINKING_PROMPTS["default"])
        
        sys_inst = prompt_template.format(
            chat_history=chat_history, 
            thoughts_history=thoughts_history, 
            user_query=query
        )
        
        prompt_parts = [query] + prompt_parts_from_files

        response_text, usage = await self.backend.generate(
            prompt_parts=prompt_parts,
            temp=0.7,
            sys_inst=sys_inst
        )
        
        print(response_text) # Debug ONLT
        
        json_str = self._extract_json_from_response(response_text)
        thought_json = json5.loads(json_str)
        
        return thought_json, usage

    async def synthesize_stream(
        self,
        query: str,
        mode: str,
        chat_history: str,
        thoughts_history: str,
        custom_instructions: str,
        prompt_parts_from_files: List[Any]
    ) -> AsyncGenerator[str, None]:
        print("\n--- [EGO_SYNTH_STREAM] НАЧАЛО СИНТЕЗА ---")
        
        prompt_template = self.SYNTHESIS_PROMPTS.get(mode, self.SYNTHESIS_PROMPTS["default"])
        
        sys_inst = prompt_template.format(
            custom_instructions=custom_instructions or "",
            chat_history=chat_history,
            thoughts_history=thoughts_history,
            user_query=query,
        )

        prompt_parts = [query] + prompt_parts_from_files

        try:
            async for chunk in self.backend.generate_stream(
                prompt_parts=prompt_parts,
                temp=0.8,
                sys_inst=sys_inst
            ):
                print(f"--- [EGO_SYNTH_STREAM] ПОЛУЧЕН КУСОК ОТ БЭКЕНДА: {chunk!r} ---")
                yield chunk
                
        except Exception as e:
            import traceback
            error_message = f"--- КРИТИЧЕСКАЯ ОШИБКА СИНТЕЗА: {e} ---\n{traceback.format_exc()}"
            print(error_message)
            yield f'{{"error": "{str(e)}"}}'

        print("--- [EGO_SYNTH_STREAM] КОНЕЦ СИНТЕЗА ---")