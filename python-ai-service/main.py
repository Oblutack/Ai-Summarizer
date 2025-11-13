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
    summary = ""

    if page_limit > 0:
        text_splitter = RecursiveCharacterTextSplitter(chunk_size=4000, chunk_overlap=200)
        chunks = text_splitter.create_documents([text])
        
        map_prompt = "Summarize the following text concisely, focusing on the key points:\n\n---\n\n{text}"
        
        tasks = []
        for chunk in chunks:
            prompt = map_prompt.format(text=chunk.page_content)
            tasks.append(llm.ainvoke(prompt))
            
        intermediate_summaries = await asyncio.gather(*tasks)
        
        combined_summaries = "\n\n".join([res.content for res in intermediate_summaries if res.content])

        if not combined_summaries:
            raise ValueError("Failed to generate intermediate summaries from the document.")

        target_words = page_limit * 250
        reduce_prompt = f"""Condense and combine the following summaries into a single, well-structured text of about {target_words} words.
        **Format the entire output strictly as Markdown.**
        Use headings (#, ##), bullet points (*), and bold text (**) to organize the information clearly and improve readability.
        
        ---
        
        {combined_summaries}"""
        
        final_response = await llm.ainvoke(reduce_prompt)
        summary = final_response.content

    else:
        prompt = f"""Provide a summary of the following text in about {word_count} words.
        **Format the output strictly as Markdown.**
        Use headings, bullet points, and bold text where appropriate to structure the key information.
        
        ---
        
        {text}"""
        response = await llm.ainvoke(prompt)
        summary = response.content

    if not summary or not summary.strip():
        raise ValueError("The model returned an empty or invalid summary.")
        
    return summary