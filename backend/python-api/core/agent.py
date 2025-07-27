import json5
import markdown_it
from typing import List, Dict, Any, cast, Union, Tuple, AsyncGenerator
from PIL import Image

from .tools import Tool
from .llm_backend import LLMBackend

def markdown_to_structured_json(md_text: str) -> List[Dict[str, Any]]:
    md = markdown_it.MarkdownIt()
    tokens = md.parse(md_text)
    return [{"type": "markdown", "content": md_text}]

class EGO:
    def __init__(self, backend: LLMBackend, tools: List[Tool], max_thoughts: int = 100, max_retries: int = 3):
        self.backend = backend
        self.tools: Dict[str, Tool] = {tool.name: tool for tool in tools}
        self.max_thoughts = max_thoughts
        self.max_retries = max_retries
        self.search_limit = 5

    def _extract_json_from_response(self, text: str) -> str:
        try:
            first_brace = text.find('{')
            if first_brace == -1:
                return "" 
            
            last_brace = text.rfind('}')
            if last_brace == -1 or last_brace < first_brace:
                return "" 
            json_str = text[first_brace : last_brace + 1]
            return json_str.strip()
        except Exception:
            return ""

    async def _run_single_thought_step(self, prompt_parts, sys_inst):
        final_prompt_for_thought = [sys_inst] + prompt_parts
        
        response_text, usage_dict = await self.backend.generate(
            prompt_parts=final_prompt_for_thought,
            temp=0.7,
            sys_inst=None
        )
        print(response_text)
        return response_text, usage_dict

    async def _run_egosynth_stream(
        self,
        prompt_parts: List[Union[str, Image.Image]],
        sys_inst: str
    ) -> AsyncGenerator[Dict[str, str], None]:
        print("\n--- [EGO_SYNTH_STREAM] НАЧАЛО СИНТЕЗА ---")
        final_prompt_for_synthesis = [sys_inst] + prompt_parts
        
        try:
            async for chunk in self.backend.generate_stream(
                prompt_parts=final_prompt_for_synthesis,
                temp=0.8,
                sys_inst=None
            ):
                print(f"--- [EGO_SYNTH_STREAM] ПОЛУЧЕН КУСОК ОТ БЭКЕНДА: {chunk!r} ---")
                yield chunk
                
        except Exception as e:
            import traceback
            error_message = f"--- КРИТИЧЕСКАЯ ОШИБКА СИНТЕЗА: {e} ---\n{traceback.format_exc()}"
            print(error_message)
            yield {"error": str(e)}

        print("--- [EGO_SYNTH_STREAM] КОНЕЦ СИНТЕЗА ---")