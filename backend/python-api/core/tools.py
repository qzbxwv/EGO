import wikipediaapi
import asyncio
import os
import docker
import tempfile
import re
import requests
from bs4 import BeautifulSoup
from utils.logger import logger
import sympy
from sympy import pi, E
import traceback


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
        from .prompts import EGO_SEARCH_PROMPT_EN
        
        print(f"--- EGO SEARCH QUERY: {query} ---")
        
        egosearch_tool = types.Tool(google_search=types.GoogleSearch()) 
        url_context_tool = types.Tool(url_context = types.UrlContext)
        response_text, _ = await self.backend.generate(
            prompt_parts=[query],
            temp=0.1, 
            sys_inst=EGO_SEARCH_PROMPT_EN, 
            tools=[egosearch_tool, url_context_tool]
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
        from .prompts import ALTER_EGO_PROMPT_EN
        print(f"--- ALTER TAKES OVER EGO WITH QUERY: {query} ---")
        
        response_text, _ = await self.backend.generate(
            prompt_parts=[query],
            temp=0.9, 
            sys_inst=ALTER_EGO_PROMPT_EN
        )
        print(f"--- ALTER RESPONSE: {response_text} ---")
        return response_text

# --- TOOLS WITHOUT BACKEND ---

class EgoCalc(Tool):
    def __init__(self):
        super().__init__(
            name="EgoCalc",
            desc="Выполняет математические вычисления используя SymPy"
        )
        
    async def use(self, query: str) -> str:
        print(f"--- EGO, CALC! with query: {query} ---")
        try:
            expr = sympy.sympify(query)
            if expr.is_Number:
                result = float(expr)
                
            else:
                result = expr
                
            return str(result)
        except (sympy.SympifyError, SyntaxError) as e:
            return f"Ошибка при парсинге или вычислении выражения: {e}. Проверь синтаксис или используй только математические функции и символы, поддерживаемые SymPy (например, sin, cos, log, sqrt, pi, E)." 
        except Exception as e:
            return f"Неожиданная ошибка: {e}"

class EgoCode(Tool):
    def __init__(self):
        super().__init__(
            name="EgoCode",
            desc="Выполняет Python код в безопасной песочнице EgoBox. Доступные библиотеки: NumPy, SciPy, SymPy."
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
            return "Execution failed: Docker client is not available."
        
        tmp_file = None
        try:
            tmp_file = tempfile.NamedTemporaryFile(
                mode='w+',
                dir=self.sandbox_dir_in_container,
                suffix='.py',
                delete=False,
                encoding='utf-8'
            )
            tmp_file.write(code_string)
            tmp_file.flush()
            tmp_file_path = tmp_file.name
            filename = os.path.basename(tmp_file_path)
            
            container_script_path = f"/sandbox/{filename}"
            volume_name = "sandbox_tmp_data"

            print(f"--- EgoCode: Running script {filename} in egobox container ---")
            
            container = self.docker_client.containers.run(
                image="egobox:latest",
                command=["python", container_script_path],
                volumes={
                    volume_name: {'bind': '/sandbox', 'mode': 'ro'}
                },
                detach=True, 
                mem_limit="256m",
                cpu_shares=512,
                network_disabled=True
            )

            result = container.wait(timeout=30)
            exit_code = result.get('StatusCode', -1)
            
            output = container.logs().decode('utf-8', errors='replace').strip()
            
            container.remove()

            if exit_code == 0:
                if not output:
                    return "Code executed successfully with no output."
                return f"Code executed successfully.\n--- OUTPUT ---\n{output}"
            else:
                return f"Code execution failed with exit code {exit_code}.\n--- ERROR LOG ---\n{output}"

        except Exception as e:
            print(f"!!! Docker ERROR in EgoCode: {traceback.format_exc()} !!!")
            return f"Docker infrastructure ERROR: {e}"
        finally:
            if tmp_file and os.path.exists(tmp_file.name):
                os.remove(tmp_file.name)

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