FROM python:3.11-slim

RUN pip install numpy sympy scipy

WORKDIR /sandbox

CMD ["python"]