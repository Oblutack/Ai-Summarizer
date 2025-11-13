import os
import tempfile
from fastapi import FastAPI, UploadFile, File, Form, HTTPException, Query
from dotenv import load_dotenv
from pydantic import BaseModel
import asyncio 

from langchain_openai import ChatOpenAI
from langchain_community.document_loaders import PyPDFLoader
from langchain.docstore.document import Document as LangchainDocument
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain.chains.summarize import load_summarize_chain

load_dotenv()

app = FastAPI(title="AI Summarizer Service")

class TextPayload(BaseModel):
    text: str

llm = ChatOpenAI(
    model="llama-3.1-8b-instant", 
    api_key=os.getenv("GROQ_API_KEY"), 
    base_url="https://api.groq.com/openai/v1"
)

@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}


@app.post("/summarize")
async def summarize_file(file: UploadFile = File(...), word_count: int = Form(150), page_limit: int = Form(0)):
    temp_pdf_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as temp_pdf:
            content = await file.read()
            temp_pdf.write(content)
            temp_pdf_path = temp_pdf.name
        
        loader = PyPDFLoader(temp_pdf_path)
        docs = loader.load()
        
        full_document_text = "\n".join([doc.page_content for doc in docs])
        
        summary = await process_summary(full_document_text, word_count, page_limit)

        return {"filename": file.filename, "summary": summary}
    
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"An error occurred: {str(e)}")
    
    finally:
        if temp_pdf_path and os.path.exists(temp_pdf_path):
            os.unlink(temp_pdf_path)

@app.post("/summarize-text")
async def summarize_text(payload: TextPayload, word_count: int = Query(150), page_limit: int = Query(0)):
    try:
        summary = await process_summary(payload.text, word_count, page_limit)
        return {"summary": summary}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


async def process_summary(text: str, word_count: int, page_limit: int) -> str:

    docs = [LangchainDocument(page_content=text)]
    
    summary = ""

    if page_limit > 0:

        text_splitter = RecursiveCharacterTextSplitter(chunk_size=4000, chunk_overlap=200)
        chunks = text_splitter.split_documents(docs)

        chain = load_summarize_chain(llm, chain_type="refine")
        
        chain.refine_prompt.template = (
            "Your job is to produce a final summary\n"
            "We have provided an existing summary up to a certain point: {existing_answer}\n"
            "We have the opportunity to refine the existing summary"
            "(only if needed) with some more context below.\n"
            "------------\n"
            "{text}\n"
            "------------\n"
            "Given the new context, refine the original summary."
            "The final summary should be **formatted strictly as Markdown**, using headings, bullet points, and bold text."
        )

        summary_result = await chain.ainvoke(chunks)
        summary = summary_result.get('output_text', '')

    else:
        
        chain = load_summarize_chain(llm, chain_type="stuff")
        
        chain.llm_chain.prompt.template = (
            f"Provide a summary of the following text in about {word_count} words.\n"
            "**Format the output strictly as Markdown**, using headings, bullet points, and bold text where appropriate.\n\n"
            "---\n\n"
            "{text}"
        )

        summary_result = await chain.ainvoke(docs)
        summary = summary_result.get('output_text', '')

    if not summary or not summary.strip():
        raise ValueError("The model returned an empty or invalid summary.")
        
    return summary