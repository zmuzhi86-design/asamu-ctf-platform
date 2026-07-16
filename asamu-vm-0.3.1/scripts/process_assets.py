from collections import deque
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
RAW = ROOT / "apps" / "web" / "public" / "assets" / "raw"
OUT = ROOT / "apps" / "web" / "public" / "assets" / "processed"


def find(name: str) -> Path:
    matches = [path for path in RAW.rglob(name) if "基础素材" not in path.parts]
    if not matches:
        matches = list(RAW.rglob(name))
    if not matches:
        raise FileNotFoundError(name)
    return matches[0]


def clear_edge_checkerboard(image: Image.Image) -> Image.Image:
    rgba = image.convert("RGBA")
    pixels = rgba.load()
    width, height = rgba.size
    queue: deque[tuple[int, int]] = deque()
    visited = set()

    def background(x: int, y: int) -> bool:
        red, green, blue, _ = pixels[x, y]
        return min(red, green, blue) > 218 and max(red, green, blue) - min(red, green, blue) < 14

    for x in range(width):
        queue.extend(((x, 0), (x, height - 1)))
    for y in range(height):
        queue.extend(((0, y), (width - 1, y)))

    while queue:
        x, y = queue.popleft()
        if (x, y) in visited or not background(x, y):
            continue
        visited.add((x, y))
        red, green, blue, _ = pixels[x, y]
        pixels[x, y] = (red, green, blue, 0)
        if x > 0:
            queue.append((x - 1, y))
        if x + 1 < width:
            queue.append((x + 1, y))
        if y > 0:
            queue.append((x, y - 1))
        if y + 1 < height:
            queue.append((x, y + 1))
    return rgba


def crop(sheet: Path, box: tuple[int, int, int, int], destination: str, transparent: bool = True) -> None:
    image = Image.open(sheet).crop(box)
    if transparent:
        image = clear_edge_checkerboard(image)
        alpha = image.getchannel("A")
        bounds = alpha.getbbox()
        if bounds:
            image = image.crop(bounds)
    output = OUT / destination
    output.parent.mkdir(parents=True, exist_ok=True)
    image.save(output, optimize=True)


def main() -> None:
    characters = find("角色与道具素材.png")
    directions = find("CTF 方向大图标素材.png")
    scenes_a = find("拓展1.png")
    scenes_b = find("拓展2.png")
    environments = find("动态环境状态素材.png")
    feedback = find("Flag 提交反馈素材.png")
    levels = find("用户等级  经验系统素材.png")
    teams = find("战队系统素材.png")
    backgrounds = find("页面背景  Banner 背景素材.png")
    awards = next((path for path in RAW.rglob("奖杯徽章素材.png")), None)
    competition = next((path for path in RAW.rglob("比赛专用素材.png")), None)
    empty_states = next((path for path in RAW.rglob("空状态  引导插画素材包.png")), None)

    character_boxes = {
        "assistant": (18, 42, 308, 365), "guide": (330, 55, 625, 365),
        "winner": (626, 55, 942, 365), "laptop": (942, 55, 1245, 365),
        "success": (18, 390, 310, 705), "flag": (332, 390, 620, 705),
        "chat": (640, 390, 930, 705), "helper": (950, 390, 1235, 705),
        "student": (30, 720, 305, 1008), "student-laptop": (320, 720, 625, 1008),
        "cheer": (640, 720, 930, 1008), "robot-dog": (950, 720, 1235, 1015),
    }
    for name, box in character_boxes.items():
        crop(characters, box, f"characters/{name}.png")

    direction_names = ["web", "pwn", "reverse", "crypto", "misc", "forensics", "iot", "mobile", "cloud", "ai"]
    for index, name in enumerate(direction_names):
        column = index % 5
        row = index // 5
        crop(directions, (column * 250, row * 400, min(1254, (column + 1) * 250 + 8), min(1254, (row + 1) * 400 + 10)), f"category-icons/{name}.png")

    scene_boxes = [(0, 0, 420, 445), (410, 0, 840, 445), (830, 0, 1254, 445), (0, 420, 420, 875), (410, 420, 840, 875), (830, 420, 1254, 875), (290, 830, 960, 1254)]
    scene_names = ["web", "pwn", "reverse", "crypto", "misc", "forensics", "iot"]
    for name, box in zip(scene_names, scene_boxes):
        crop(scenes_a, box, f"category-scenes/{name}.png")
    second_names = ["mobile", "cloud", "ai", "blockchain", "osint", "ics"]
    for name, box in zip(second_names, scene_boxes[:6]):
        crop(scenes_b, box, f"category-scenes/{name}.png")

    env_names = ["idle", "starting", "running", "expiring", "expired", "resetting", "failed", "maintenance"]
    for index, name in enumerate(env_names):
        column, row = index % 4, index // 4
        crop(environments, (column * 313, row * 570, min(1254, (column + 1) * 313 + 2), min(1254, (row + 1) * 570 + 10)), f"environment/{name}.png")

    feedback_names = ["success", "error", "first-blood", "solved", "duplicate", "too-fast", "format-error", "locked", "review", "processing", "expired", "achievement"]
    for index, name in enumerate(feedback_names):
        column, row = index % 4, index // 4
        crop(feedback, (column * 313, row * 400, min(1254, (column + 1) * 313 + 2), min(1254, (row + 1) * 400 + 8)), f"flag-feedback/{name}.png")

    banner_names = ["grid-panel", "night-lab", "campus", "floating-academy", "competition-stage", "learning-map", "operations-lab", "academy"]
    banner_boxes = []
    for row in range(4):
        for column in range(2):
            banner_boxes.append((20 + column * 625, 25 + row * 310, 610 + column * 625, 325 + row * 310))
    for name, box in zip(banner_names, banner_boxes):
        crop(backgrounds, box, f"banners/{name}.png", transparent=False)

    if awards:
        award_names = ["gold-cup", "silver-cup", "bronze-cup", "laurel", "gold-medal", "silver-medal", "bronze-medal", "blood-badge", "security-badge", "champion-cup", "three-stars", "coin", "star-banner", "writeup-badge", "verified-badge"]
        award_boxes = [
            (0, 0, 313, 315), (313, 0, 626, 315), (626, 0, 939, 315), (939, 0, 1254, 315),
            (0, 300, 313, 625), (313, 300, 626, 625), (626, 300, 939, 625), (939, 300, 1254, 625),
            (0, 610, 360, 945), (360, 610, 820, 945), (820, 610, 1254, 945),
            (0, 920, 313, 1254), (313, 920, 626, 1254), (626, 920, 939, 1254), (939, 920, 1254, 1254),
        ]
        for name, box in zip(award_names, award_boxes):
            crop(awards, box, f"achievements/{name}.png")

    if competition:
        crop(competition, (40, 180, 400, 460), "competitions/podium.png")
        crop(competition, (400, 180, 680, 460), "competitions/trophy-stage.png")
        crop(competition, (650, 0, 930, 210), "competitions/countdown.png")
        crop(competition, (0, 0, 280, 220), "competitions/first-blood.png")
        crop(competition, (35, 1000, 420, 1254), "competitions/award-stage.png")

    team_boxes = {
        "flag": (0, 0, 190, 210), "banner": (190, 0, 385, 210), "captain": (620, 210, 875, 420),
        "avatar-frame": (0, 390, 210, 585), "trophy": (1010, 570, 1254, 760),
        "notice-board": (390, 900, 650, 1120), "base": (640, 900, 940, 1140), "honor": (790, 580, 1015, 780),
    }
    for name, box in team_boxes.items():
        crop(teams, box, f"teams/{name}.png")

    crop(levels, (0, 0, 320, 260), "achievements/level-badges.png")
    crop(levels, (760, 0, 1254, 300), "achievements/experience-bars.png")
    crop(levels, (910, 930, 1254, 1254), "achievements/treasure.png")

    if empty_states:
        empty_names = ["empty", "success", "error", "maintenance", "search", "network", "locked", "coming-soon"]
        for index, name in enumerate(empty_names):
            column, row = index % 4, index // 4
            crop(empty_states, (column * 313, row * 570, min(1254, (column + 1) * 313 + 2), min(1254, (row + 1) * 570 + 10)), f"empty-states/{name}.png")

    print(f"Processed assets written to {OUT}")


if __name__ == "__main__":
    main()
