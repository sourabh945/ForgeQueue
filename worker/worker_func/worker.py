from typing import TypedDict

class WorkerResult(TypedDict):
    JobId: str
    Status: str
    ImagePath: str


def worker_(job_id: str, img_path: str, type: str) -> WorkerResult:
    """
    Executes the worker function for the given job and image path.
    """
    print(f'Worker {job_id} started on {img_path}')

    try:
        # write the worker here

        # worker end

        return {"JobId": job_id, "Status": "Done", "ImagePath": img_path}
    except Exception as e:
        print(f"Worker {job_id} failed: {e}")
        return {"JobId": job_id, "Status": "failed", "ImagePath": img_path}
