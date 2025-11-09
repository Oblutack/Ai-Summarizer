import os
import tempfile
from fastapi import FastAPI, UploadFile, File, HTTPException
from dotenv import load_dotenv

from langchain_openai import ChatOpenAI
from langchain.document_loaders import PyPDFLoader
from langchain.chains.summarize import load_summarize_chain

load_dotenv()

app = FastAPI(title="AI Summarizer Service")


llm = ChatOpenAI(
    model="llama3",  
    base_url="http://host.docker.internal:11434/v1",
    api_key="NA"
)


@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}

@app.post("/summarize")
async def summarize_file(file: UploadFile = File(...)):

    temp_pdf_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as temp_pdf:
            content = await file.read()
            temp_pdf.write(content)
            temp_pdf_path = temp_pdf.name
        
        loader = PyPDFLoader(temp_pdf_path)
        docs = loader.load()
        
        chain = load_summarize_chain(llm, chain_type="stuff")
        
        summary = chain.run(docs)

        return {"filename": file.filename, "summary": summary}
    
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"An error occurred: {str(e)}")
    
    finally:
        if temp_pdf_path and os.path.exists(temp_pdf_path):
            os.unlink(temp_pdf_path)