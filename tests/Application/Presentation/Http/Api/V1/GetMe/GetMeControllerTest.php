<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Tests\Application\Presentation\Http\Api\V1\GetMe;

use Symfony\Bundle\FrameworkBundle\Test\WebTestCase;

class GetMeControllerTest extends WebTestCase
{
    public function test(): void
    {
        $client = static::createClient();

        $crawler = $client->request('GET', '/api/v1/me');

        self::assertResponseStatusCodeSame(401);
    }

}
