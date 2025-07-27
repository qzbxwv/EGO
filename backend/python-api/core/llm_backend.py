import os
import asyncio
import itertools
from abc import ABC, abstractmethod
from typing import List, Optional, Any, Tuple, Union, AsyncGenerator

from google import genai
from google.genai import types
from PIL import Image

class LLMBackend(ABC):
    @abstractmethod
    async def generate(self, *args, **kwargs) -> Tuple[str, Optional[dict]]:
        raise NotImplementedError

    @abstractmethod
    async def generate_stream(self, *args, **kwargs) -> AsyncGenerator[str, None]:
        yield; return

    @abstractmethod
    async def upload_file(self, *args, **kwargs) -> Any:
        raise NotImplementedError

    @abstractmethod
    async def get_client_for_session(self) -> Any:
        raise NotImplementedError


class GeminiBackend(LLMBackend):
    def __init__(self):
        keys_str = os.getenv("GEMINI_BACKEND_API_KEYS")
        if not keys_str:
            raise ValueError("Переменная окружения GEMINI_BACKEND_API_KEYS не найдена")

        self.api_keys = keys_str.split(",")
        if not self.api_keys:
            raise ValueError("Не предоставлено ни одного API-ключа Gemini")

        self.clients_pool = [(key, genai.Client(api_key=key)) for key in self.api_keys]
        self.client_rotator = itertools.cycle(self.clients_pool)
        print(f"--- ИНИЦИАЛИЗАЦИЯ GeminiBackend (google-genai SDK) с {len(self.clients_pool)} клиентами ---")

    async def get_client_for_session(self) -> Tuple[str, genai.Client]:
        return next(self.client_rotator)

    async def upload_file(self, file_data: Any, client: genai.Client, api_key_for_log: str) -> Any: # Возвращаем Any
        import tempfile

        print(f"--- FILE API: Загрузка файла '{file_data.file_name}' с помощью ключа ...{api_key_for_log[-4:]} ---")
        suffix = os.path.splitext(file_data.file_name)[1]

        with tempfile.NamedTemporaryFile(delete=False, suffix=suffix, mode='wb') as tmp_file:
            tmp_file.write(file_data.raw_bytes)
            tmp_file_path = tmp_file.name

        try:
            uploaded_file_obj = await asyncio.to_thread(
                client.files.upload,
                file=tmp_file_path
            )
            print(f"--- FILE API: Файл '{uploaded_file_obj.display_name or file_data.file_name}' успешно загружен. URI: {uploaded_file_obj.name} ---")
            return uploaded_file_obj
        finally:
            os.remove(tmp_file_path)

    async def generate_stream(
        self,
        prompt_parts: List[Any],
        temp: float,
        sys_inst: Optional[str] = None,
        client_override: Optional[Tuple[str, genai.Client]] = None,
    ):
        if client_override:
            api_key, client = client_override
        else:
            api_key, client = await self.get_client_for_session()

        print(f"--- GEMINI STREAM: Используется клиент (API ключ ...{api_key[-4:]}) ---")

        config = types.GenerateContentConfig(temperature=temp)
        if sys_inst:
            config.system_instruction = sys_inst

        response_stream = await client.aio.models.generate_content_stream(
            model='gemini-2.5-flash',
            contents=prompt_parts,
            config=config,
        )
        async for chunk in response_stream:
            if chunk.text:
                yield chunk.text

    async def generate(
        self,
        prompt_parts: List[Any],
        temp: float,
        sys_inst: Optional[str] = None,
        tools: Optional[List[types.Tool]] = None,
        client_override: Optional[Tuple[str, genai.Client]] = None,
    ) -> Tuple[str, Optional[dict]]:
        if client_override:
            api_key, client = client_override
        else:
            api_key, client = await self.get_client_for_session()

        print(f"--- GEMINI GENERATE: Используется клиент (API ключ ...{api_key[-4:]}) ---")

        config = types.GenerateContentConfig(temperature=temp)
        if sys_inst: config.system_instruction = sys_inst
        if tools: config.tools = tools

        response = await client.aio.models.generate_content(
            model='gemini-2.5-flash',
            contents=prompt_parts,
            config=config,
        )

        response_text = response.text if hasattr(response, "text") and response.text is not None else ""
        
        usage_dict = None
        if hasattr(response, 'usage_metadata') and response.usage_metadata:
            usage_metadata_obj = response.usage_metadata
            usage_dict = {
                "promptTokenCount": usage_metadata_obj.prompt_token_count,
                "candidatesTokenCount": usage_metadata_obj.candidates_token_count,
                "totalTokenCount": usage_metadata_obj.total_token_count,
            }

        return response_text, usage_dict