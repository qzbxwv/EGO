import wikipediaapi
import asyncio
import os
import docker
import tempfile
import re
import requests
from bs4 import BeautifulSoup
from utils.logger import logger


from .llm_backend import LLMBackend, GeminiBackend
from google.genai import types

class Tool:
    def __init__(self, name: str, desc: str):
        self.name = name
        self.desc = desc

    async def use(self, query: str) -> str:
        raise NotImplementedError("Родительский класс 'Tool' не предназначен для использования.")


# --- TOOLS WITH BACKEND ---
class EgoSearch(Tool):
    def __init__(self, backend: LLMBackend):
        super().__init__(
            name="EgoSearch", 
            desc="Ego использует Google Search для поиска информации."
        )
        self.backend = backend

    async def use(self, query: str) -> str:
        from .prompts import EGO_SEARCH_PROMPT_RU
        
        print(f"--- EGO SEARCH QUERY: {query} ---")
        
        egosearch_tool = types.Tool(google_search=types.GoogleSearch()) 
        response_text, _ = await self.backend.generate(
            prompt_parts=[query],
            temp=0.1, 
            sys_inst=EGO_SEARCH_PROMPT_RU, 
            tools=[egosearch_tool]
        )
        return response_text

class AlterEgo(Tool):
    def __init__(self, backend: LLMBackend):
        super().__init__(
            name="AlterEgo", 
            desc="AlterEgo берет на себя управление, чтобы проанализировать мысль."
        )
        self.backend = backend
    
    async def use(self, query: str) -> str:
        from .prompts import ALTER_EGO_PROMPT_RU
        print(f"--- ALTER TAKES OVER EGO WITH QUERY: {query} ---")
        
        response_text, _ = await self.backend.generate(
            prompt_parts=[query],
            temp=0.9, 
            sys_inst=ALTER_EGO_PROMPT_RU
        )
        print(f"--- ALTER RESPONSE: {response_text} ---")
        return response_text

# --- TOOLS WITHOUT BACKEND ---

class EgoCalc(Tool):
    def __init__(self):
        super().__init__(
            name="EgoCalc", 
            desc="Ego выполняет математические вычисления. Поддерживаются базовые операции."
        )
    
    def _is_safe_expr(self, expr: str) -> bool:
        return bool(re.fullmatch(r'[0-9\.\+\-\*\/\(\)\s]+', expr))

    async def use(self, query: str) -> str:
        print(f"--- EGO, CALC! with query: {query} ---")
        if not self._is_safe_expr(query):
            return "Ошибка: выражение содержит недопустимые символы."
        try:
            # Безопасное вычисление
            result = eval(query, {"__builtins__": {}}, {})
            return str(result)
        except Exception as e:
            return f"Ошибка при вычислении: {e}"
        
class EgoCode(Tool):
    def __init__(self):
        super().__init__(
            name="EgoCode",
            desc="Выполняет Python код в безопасной песочнице EgoBox с ограниченными библиотеками (NumPy, SciPy, SymPy)."
        )
        try:
            self.docker_client = docker.from_env()
            self.docker_client.ping()
            print("--- Docker-client EgoBox READY. ---")
        except Exception as e:
            print(f"--- Docker-client NOT READY: {e} ---")
            self.docker_client = None

        self.sandbox_dir_in_container = "/app/sandbox_tmp"
        os.makedirs(self.sandbox_dir_in_container, exist_ok=True)

    def _execute_in_docker_sync(self, code_string: str) -> str:
        if not self.docker_client:
            return "--- EGOBOX IS UNREACHABLE ---"

        with tempfile.NamedTemporaryFile(mode='w+', dir=self.sandbox_dir_in_container, suffix='.py', delete=True) as tmp_file:
            tmp_file.write(code_string)
            tmp_file.flush()

            path_in_container = tmp_file.name
            path_on_host = os.path.abspath(path_in_container.replace("/app", "./backend/python-api"))
            filename = os.path.basename(path_in_container)
            container_path = f"/sandbox/{filename}"
            volume_mapping = {path_on_host: {'bind': container_path, 'mode': 'ro'}}
            
            print(f"--- EgoCode: Host path: {path_on_host} ---")
            print(f"--- EgoCode: Container path: {container_path} ---")

            try:
                output = self.docker_client.containers.run(
                    image="egobox:latest",
                    command=["python", container_path],
                    volumes=volume_mapping,
                    remove=True,
                    mem_limit="256m",
                    cpu_shares=512,
                    network_disabled=True 
                )
                return output.decode('utf-8', errors='replace').strip()
            except Exception as e:
                return f"Docker ERROR: {e}"

    async def use(self, query: str) -> str:
        print(f"--- EGO, CODE! with query: {query} ---")
        return await asyncio.to_thread(self._execute_in_docker_sync, query)

class EgoWiki(Tool):
    def __init__(self):
        super().__init__(name="EgoWiki", desc="Использует Wikipedia для поиска точной информации.")
        self.wiki_wiki = wikipediaapi.Wikipedia(
            user_agent="EGO knowledge",
            language="ru",
            extract_format=wikipediaapi.ExtractFormat.WIKI,
        )
        
    def _search_wiki_sync(self, query: str):
        try:
            wiki_page = self.wiki_wiki.page(query) 
            if wiki_page.exists():
                return wiki_page.text
            else:
                return f"Страница '{query}' на Википедии не найдена."
        except Exception as e:
             return f'Ошибка при вызове WikiAPI: {e}'

    async def use(self, query: str) -> str:
        print(f"--- WikiAPI CALL WITH QUERY: {query} ---")
        return await asyncio.to_thread(self._search_wiki_sync, query)