import os
import tempfile
from fastapi import FastAPI, UploadFile, File, Form, HTTPException, Query
from dotenv import load_dotenv
from pydantic import BaseModel

from langchain_openai import ChatOpenAI
from langchain.document_loaders import PyPDFLoader
from langchain.docstore.document import Document as LangchainDocument


load_dotenv()

app = FastAPI(title="AI Summarizer Service")

class TextPayload(BaseModel):
    text: str

llm = ChatOpenAI(
    model="llama3",  
    base_url="http://host.docker.internal:11434/v1",
    api_key="NA"
)

@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}

@app.post("/summarize")
async def summarize_file(file: UploadFile = File(...), word_count: int = Form(150)):
    temp_pdf_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as temp_pdf:
            content = await file.read()
            temp_pdf.write(content)
            temp_pdf_path = temp_pdf.name
        
        loader = PyPDFLoader(temp_pdf_path)
        docs = loader.load()
        
        full_document_text = "\n".join([doc.page_content for doc in docs])

        prompt = f"Provide a summary of the following document in about {word_count} words:\n\n---\n\n{full_document_text}"
        
        response = llm.invoke(prompt)
        summary = response.content

        return {"filename": file.filename, "summary": summary}
    
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"An error occurred: {str(e)}")
    
    finally:
        if temp_pdf_path and os.path.exists(temp_pdf_path):
            os.unlink(temp_pdf_path)


@app.post("/summarize-text")
async def summarize_text(payload: TextPayload, word_count: int = Query(150)):
    try:

        prompt = f"Provide a summary of the following text in about {word_count} words:\n\n---\n\n{payload.text}"
        
        response = llm.invoke(prompt)
        summary = response.content

        return {"summary": summary}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
