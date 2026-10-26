import os
import streamlit as st

smtp = os.getenv("SMTP_PASSWORD")
admins = os.environ["ADMIN_EMAILS"]
db = os.environ.get("DATABASE_URL")
