import os
import tempfile
from fastapi import FastAPI, UploadFile, File, Form, HTTPException, Query
from dotenv import load_dotenv
from pydantic import BaseModel
import asyncio # Potrebno za paralelno izvršavanje

from langchain_openai import ChatOpenAI
from langchain_community.document_loaders import PyPDFLoader
from langchain.docstore.document import Document as LangchainDocument
from langchain.text_splitter import RecursiveCharacterTextSplitter

load_dotenv()

app = FastAPI(title="AI Summarizer Service")

class TextPayload(BaseModel):
    text: str

llm = ChatOpenAI(
    model="llama3",  
    base_url="http://host.docker.internal:11434/v1",
    api_key="NA",
    temperature=0,
)

@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}

# --- POTPUNO NOVA LOGIKA ZA OBA ENDPOINTA ---

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
        
        # Pozivamo našu novu, centralnu funkciju za sažimanje
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
        # Pozivamo našu novu, centralnu funkciju za sažimanje
        summary = await process_summary(payload.text, word_count, page_limit)
        return {"summary": summary}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# --- NAŠA NOVA, RUČNA "MAP REDUCE" FUNKCIJA ---
async def process_summary(text: str, word_count: int, page_limit: int) -> str:
    summary = ""

    if page_limit > 0:
        # --- MAP FAZA (ostaje ista) ---
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

        # --- REDUCE FAZA (PROMENJEN PROMPT) ---
        target_words = page_limit * 250
        # Dodajemo instrukcije za Markdown formatiranje
        reduce_prompt = f"""Condense and combine the following summaries into a single, well-structured text of about {target_words} words.
        **Format the entire output strictly as Markdown.**
        Use headings (#, ##), bullet points (*), and bold text (**) to organize the information clearly and improve readability.
        
        ---
        
        {combined_summaries}"""
        
        final_response = await llm.ainvoke(reduce_prompt)
        summary = final_response.content

    else:
        # --- LOGIKA ZA KRATAK TEKST (PROMENJEN PROMPT) ---
        # Dodajemo instrukcije za Markdown formatiranje
        prompt = f"""Provide a summary of the following text in about {word_count} words.
        **Format the output strictly as Markdown.**
        Use headings, bullet points, and bold text where appropriate to structure the key information.
        
        ---
        
        {text}"""
        response = await llm.ainvoke(prompt)
        summary = response.content

    # Finalna provera (ostaje ista)
    if not summary or not summary.strip():
        return "Could not generate a summary. The provided text might be too short, unclear, or contain unsupported content."
        
    return summary