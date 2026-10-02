import html2pdf from "html2pdf.js";

// Renders an element to a PDF and downloads it. The texture overlay is hidden in the capture so it
// doesn't get baked into the page.
export function saveElementAsPdf(element: HTMLElement, filename: string) {
  const options = {
    margin: [0.5, 0.5, 0.5, 0.5] as [number, number, number, number],
    filename,
    image: { type: "jpeg" as const, quality: 0.98 },
    html2canvas: {
      scale: 2,
      useCORS: true,
      backgroundColor: "#F5F0E6",
      onclone: (clonedDocument: Document) => {
        const texture = clonedDocument.querySelector(".texture-div-for-pdf-export");
        if (texture instanceof HTMLElement) {
          texture.style.display = "none";
        }
      },
    },
    jsPDF: {
      unit: "in" as const,
      format: "letter" as const,
      orientation: "portrait" as const,
    },
  };

  return html2pdf().from(element).set(options).save();
}
