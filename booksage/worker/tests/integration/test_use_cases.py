# Chunking
from booksage.chunking import ChonkersChunker, FreeChunker
from booksage.domain import DocumentMetadata, QueryContext

# ETL
from booksage.etl.models import RawDocument

# Generation
from booksage.generation import AgenticGenerator

# Infrastructure
from booksage.infrastructure import MilvusVectorStore

# Retrieval
from booksage.retrieval import ColBERTV2Engine, FusionRetriever, LightRAGEngine, RAPTOREngine


def test_use_case_1_standard_book_ingestion():
    """
    ユースケース1: 標準的なプレーンテキスト主体の書籍インジェスト
    PyMuPDF -> FreeChunker -> MilvusVectorStore
    """
    # 2. メタデータを付与してPDFをパース (Mocked)
    metadata = DocumentMetadata(
        book_id="book-101", title="Introduction to LLMs", author="AI Author"
    )
    raw_doc = RawDocument(
        document_id="test-101", text="Mocked content for integration tests.", metadata=metadata
    )

    # 検証：メタデータが保持されたまま抽出されているか
    assert raw_doc.metadata.book_id == "book-101"
    assert len(raw_doc.text) > 0

    # 3. 意味論的境界をベースにしたFreeChunkerでチャンキング
    chunker = FreeChunker(chunk_size=500, chunk_overlap=50)
    chunks = chunker.create_chunks(raw_doc)

    # 検証：適切に分割され、各チャンクにメタデータが継承されているか
    # （2-Levelインデックス構造の要件）
    assert len(chunks) > 0
    for chunk in chunks:
        assert chunk.document_id == raw_doc.document_id
        assert chunk.metadata == metadata

    # 4. VectorStoreへの格納（モック）
    store = MilvusVectorStore(collection_name="test_books", dimension=1536)
    store.add_chunks(chunks)


def test_use_case_2_complex_document_ingestion():
    """
    ユースケース2: 数式・表を含む複雑なレイアウトの論文・書籍インジェスト
    Docling -> Chonkers (CDC) -> MilvusVectorStore
    """
    # 2. メタデータを付与してPDFをパース (Mocked)
    metadata = DocumentMetadata(
        book_id="paper-202",
        title="Attention Is All You Need",
        extra_attributes={"category": "research"},
    )
    raw_doc = RawDocument(
        document_id="test-202",
        text="Mocked content with [Table] and [Equation] for integration tests.",
        metadata=metadata,
    )

    # 検証：マークダウン要素（モック化）が維持されているか
    assert "[Table]" in raw_doc.text or "[Equation]" in raw_doc.text

    # 3. CDCベースの安定したハッシュを提供するChonkersChunker
    chunker = ChonkersChunker(target_chunk_size=400)
    chunks = chunker.create_chunks(raw_doc)

    # 検証：各チャンクにindex_locality_hashが存在するか（インデックス安定性の要件）
    assert len(chunks) > 0
    for chunk in chunks:
        assert chunk.index_locality_hash is not None
        assert len(chunk.index_locality_hash) > 0

    # 4. VectorStoreへの格納
    store = MilvusVectorStore(collection_name="test_papers", dimension=1536)
    store.add_chunks(chunks)


def test_use_case_3_complex_query_rag_pipeline():
    """
    ユースケース3: ユーザーの複雑な質問に対する高度な推論と回答生成
    # AgenticGenerator (CoR) -> FusionRetriever (LightRAG, RAPTOR, ColBERTv2)
    # -> AgenticGenerator (Self-RAG Critique)
    """
    # 1. フュージョン検索パイプラインの構築
    engines = [LightRAGEngine(), RAPTOREngine(), ColBERTV2Engine()]
    fusion_retriever = FusionRetriever(engines=engines)

    # 2. エージェントの生成（検索パイプラインを注入）
    agent = AgenticGenerator(retriever=fusion_retriever)

    # 3. クエリを投入
    query_context = QueryContext(
        original_query="LLMのアーキテクチャ概要と学習方法について詳しく教えて"
    )
    final_answer = agent.generate_answer(query_context)

    # 検証：CoRによってクエリが複数（Aspect 1, 2）に分解されたか
    assert len(query_context.sub_queries) >= 2
    assert "Aspect 1" in query_context.sub_queries[0]

    # 検証：回答生成とCritique（Self-RAGによる検証）が実行されているか
    assert "Verified by Self-RAG" in final_answer or "Caution" in final_answer
    assert "Based on the mock context" in final_answer
