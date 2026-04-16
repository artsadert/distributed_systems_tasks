import time
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from app.models import Base, Customer, Product, Order
from app.transactions import place_order, update_customer_email, add_product
from app.config import Config

def wait_for_db(db_url, retries=10, delay=3):
    """Ожидание готовности базы данных"""
    from sqlalchemy.exc import OperationalError
    for i in range(retries):
        try:
            engine = create_engine(db_url)
            conn = engine.connect()
            conn.close()
            print("База данных готова")
            return engine
        except OperationalError:
            print(f"Ожидание БД... попытка {i+1}/{retries}")
            time.sleep(delay)
    raise Exception("Не удалось подключиться к БД")

def main():
    db_url = Config.DATABASE_URL
    engine = wait_for_db(db_url)
    Session = sessionmaker(bind=engine)

    # Создание таблиц
    Base.metadata.create_all(engine)

    session = Session()

    # Инициализация тестовых данных (если таблицы пусты)
    if session.query(Customer).count() == 0:
        print("Добавление тестового клиента и товаров...")
        cust = Customer(FirstName="Иван", LastName="Петров", Email="ivan@example.com")
        prod1 = Product(ProductName="Ноутбук", Price=50000.00)
        prod2 = Product(ProductName="Мышь", Price=1500.00)
        session.add_all([cust, prod1, prod2])
        session.commit()
        print("Тестовые данные добавлены.")

    # Получаем ID тестовых записей
    customer = session.query(Customer).filter_by(Email="ivan@example.com").first()
    product1 = session.query(Product).filter_by(ProductName="Ноутбук").first()
    product2 = session.query(Product).filter_by(ProductName="Мышь").first()

    print("\n=== Сценарий 1: размещение заказа ===")
    try:
        items = [(product1.ProductID, 1), (product2.ProductID, 2)]
        order_id = place_order(session, customer.CustomerID, items)
        print(f"Заказ #{order_id} успешно создан. Общая сумма: {session.query(Order).get(order_id).TotalAmount} руб.")
    except Exception as e:
        print(f"Ошибка: {e}")

    print("\n=== Сценарий 2: обновление email клиента ===")
    try:
        new_email = "ivan.new@example.com"
        update_customer_email(session, customer.CustomerID, new_email)
        updated = session.query(Customer).get(customer.CustomerID)
        print(f"Email клиента обновлён: {updated.Email}")
    except Exception as e:
        print(f"Ошибка: {e}")

    print("\n=== Сценарий 3: добавление нового товара ===")
    try:
        new_product_id = add_product(session, "Клавиатура", 2500.00)
        new_product = session.query(Product).get(new_product_id)
        print(f"Добавлен товар: {new_product.ProductName} (ID {new_product.ProductID}), цена {new_product.Price} руб.")
    except Exception as e:
        print(f"Ошибка: {e}")

    session.close()
    print("\nДемонстрация завершена.")

if __name__ == "__main__":
    main()
