<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\GetMe;

readonly class GetMeDto
{
    public function __construct(
        public int $id,
        public string $email,
        public string $phone,
        public string $firstName,
        public string $lastName,
    ){}

}
