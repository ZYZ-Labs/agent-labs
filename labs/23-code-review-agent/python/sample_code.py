import os
import sys

def add(x,y):
    return x+y

def greet(name):
    message = "Hello, " + name + "!"
    print(message)

class calculator:
    def __init__(self, value):
        self.value=value
    def multiply(self, factor):
        return self.value*factor

unused_var = 42
