from sqlalchemy import Column, Integer, String, Numeric, ForeignKey, DateTime
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import relationship
from datetime import datetime

Base = declarative_base()

class Customer(Base):
    __tablename__ = 'customers'
    CustomerID = Column(Integer, primary_key=True)
    FirstName = Column(String(100), nullable=False)
    LastName = Column(String(100), nullable=False)
    Email = Column(String(200), unique=True, nullable=False)

class Product(Base):
    __tablename__ = 'products'
    ProductID = Column(Integer, primary_key=True)
    ProductName = Column(String(200), nullable=False)
    Price = Column(Numeric(10,2), nullable=False)

class Order(Base):
    __tablename__ = 'orders'
    OrderID = Column(Integer, primary_key=True)
    CustomerID = Column(Integer, ForeignKey('customers.CustomerID'), nullable=False)
    OrderDate = Column(DateTime, default=datetime.utcnow)
    TotalAmount = Column(Numeric(10,2), nullable=False, default=0)

    customer = relationship("Customer")
    items = relationship("OrderItem", cascade="all, delete-orphan", backref="order")

class OrderItem(Base):
    __tablename__ = 'orderitems'
    OrderItemID = Column(Integer, primary_key=True)
    OrderID = Column(Integer, ForeignKey('orders.OrderID'), nullable=False)
    ProductID = Column(Integer, ForeignKey('products.ProductID'), nullable=False)
    Quantity = Column(Integer, nullable=False)
    Subtotal = Column(Numeric(10,2), nullable=False)

    product = relationship("Product")
