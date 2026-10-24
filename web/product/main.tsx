import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ProductPage } from "@/product/product-page";
import "./product.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ProductPage />
  </StrictMode>
);
