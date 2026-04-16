import os
from dotenv import load_dotenv

# Загружаем переменные из .env (для локального запуска)
load_dotenv()

class Config:
    DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://user:password@localhost:5432/store")
