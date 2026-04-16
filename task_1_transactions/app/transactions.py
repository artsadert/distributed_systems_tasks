from sqlalchemy.orm import Session
from app.models import Order, OrderItem, Customer, Product

def place_order(session: Session, customer_id: int, items):
    """
    Сценарий 1: размещение заказа.
    items: список кортежей (product_id, quantity)
    """
    try:
        # Создаём заказ с временной суммой 0
        order = Order(CustomerID=customer_id, TotalAmount=0)
        session.add(order)
        session.flush()  # получаем OrderID

        total = 0
        for product_id, quantity in items:
            product = session.query(Product).filter(Product.ProductID == product_id).first()
            if not product:
                raise ValueError(f"Товар с ID {product_id} не найден")
            subtotal = product.Price * quantity
            order_item = OrderItem(
                OrderID=order.OrderID,
                ProductID=product_id,
                Quantity=quantity,
                Subtotal=subtotal
            )
            session.add(order_item)
            total += subtotal

        # Обновляем общую сумму заказа
        order.TotalAmount = total
        session.commit()
        return order.OrderID
    except Exception:
        session.rollback()
        raise

def update_customer_email(session: Session, customer_id: int, new_email: str):
    """Сценарий 2: атомарное обновление email клиента"""
    try:
        customer = session.query(Customer).filter(Customer.CustomerID == customer_id).first()
        if not customer:
            raise ValueError(f"Клиент с ID {customer_id} не найден")
        customer.Email = new_email
        session.commit()
    except Exception:
        session.rollback()
        raise

def add_product(session: Session, name: str, price: float):
    """Сценарий 3: атомарное добавление нового товара"""
    try:
        product = Product(ProductName=name, Price=price)
        session.add(product)
        session.commit()
        return product.ProductID
    except Exception:
        session.rollback()
        raise
