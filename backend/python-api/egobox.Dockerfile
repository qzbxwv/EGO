FROM python:3.11-slim
RUN pip install --no-cache-dir numpy scipy sympy
RUN useradd -m -s /bin/bash sandboxuser
USER sandboxuser
WORKDIR /sandbox
CMD ["/bin/bash"]